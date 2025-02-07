package updater

import (
	"context"
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/pkg/backoff"
	"github.com/unbasical/doras/pkg/client/updater/fetcher"
	"github.com/unbasical/doras/pkg/client/updater/statemanager"
	"github.com/unbasical/doras/pkg/client/updater/updaterstate"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"os"
	"path"
	"path/filepath"

	"github.com/unbasical/doras/pkg/client/edgeapi"
)

type clientOpts struct {
	RemoteURL          string
	OutputDirectory    string
	InternalDirectory  string
	DockerConfigPath   string
	AcceptedAlgorithms []string
}

// NewClient creates a new Doras update client with the provided options.
func NewClient(options ...func(*Client)) (*Client, error) {
	client := &Client{
		opts: clientOpts{
			OutputDirectory:   ".",
			InternalDirectory: os.TempDir(),
			DockerConfigPath:  filepath.Join(os.Getenv("HOME"), ".docker", "config.json"),
		},
		backoff: backoff.DefaultBackoff(),
	}

	for _, option := range options {
		option(client)
	}
	store, err := credentials.NewStore(client.opts.DockerConfigPath, credentials.StoreOptions{
		DetectDefaultNativeStore: true,
	})
	if err != nil {
		return nil, err
	}
	credentialFunc := credentials.Credential(store)
	c, err := edgeapi.NewEdgeClient(client.opts.RemoteURL, true, credentialFunc)
	if err != nil {
		return nil, err
	}
	client.edgeClient = c
	initialState := updaterstate.State{
		Version:        "1",
		ArtifactStates: make(map[string]string),
	}
	err = os.MkdirAll(client.opts.InternalDirectory, 0755)
	if err != nil {
		log.WithError(err).Error("Failed to create output directory")
		return nil, err
	}
	statePath := path.Join(client.opts.InternalDirectory, "doras-state.json")
	stateManager, err := statemanager.NewFromDisk(initialState, statePath)
	if err != nil {
		return nil, err
	}
	client.state = stateManager
	fetcherDir := path.Join(client.opts.InternalDirectory, "fetcher")
	err = os.Mkdir(fetcherDir, 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}

	storageSource := fetcher.NewRepoStorageSource(false, credentialFunc)
	client.reg = fetcher.NewArtifactLoader(fetcherDir, storageSource)
	return client, nil
}

// WithRemoteURL adds the URL of a Doras server to the client configuration.
func WithRemoteURL(remoteURL string) func(*Client) {
	return func(c *Client) {
		c.opts.RemoteURL = remoteURL
	}
}

// WithOutputDirectory adds a directory to which output files are written.
func WithOutputDirectory(outputDirectory string) func(*Client) {
	return func(c *Client) {
		c.opts.OutputDirectory = outputDirectory
	}
}

// WithInternalDirectory sets the client configurations local working directory.
// It stores things such as the updaters internal state.
func WithInternalDirectory(internalDirectory string) func(*Client) {
	return func(c *Client) {
		c.opts.InternalDirectory = internalDirectory
	}
}

// WithDockerConfigPath adds a path to a docker config file to the client configuration.
// The file is used to load locally stored registry credentials.
func WithDockerConfigPath(dockerConfigPath string) func(*Client) {
	return func(c *Client) {
		c.opts.DockerConfigPath = dockerConfigPath
	}
}

// WithAcceptedAlgorithms allows to restrict the set of accepted algorithms which are used to build a delta.
func WithAcceptedAlgorithms(acceptedAlgorithms []string) func(*Client) {
	return func(c *Client) {
		c.opts.AcceptedAlgorithms = acceptedAlgorithms
	}
}

// WithContext adds a ctx to the client (e.g. for cancellation).
func WithContext(ctx context.Context) func(*Client) {
	return func(c *Client) {
		c.ctx = ctx
	}
}

// WithBackoffStrategy adds ...
func WithBackoffStrategy(b backoff.BackoffStrategy) func(*Client) {
	return func(c *Client) {
		c.backoff = b
	}
}
