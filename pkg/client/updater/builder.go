package updater

import (
	"context"
	"errors"
	"fmt"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"github.com/unbasical/doras/pkg/backoff"
	"github.com/unbasical/doras/pkg/client/updater/fetcher"
	"github.com/unbasical/doras/pkg/client/updater/statemanager"
	"github.com/unbasical/doras/pkg/client/updater/updaterstate"
	"oras.land/oras-go/v2/registry/remote/auth"
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
	CredFuncs          []auth.CredentialFunc
	InsecureAllowHTTP  bool
	KeepOldDir         bool
}

// NewClient creates a new Doras update client with the provided options.
func NewClient(options ...func(*Client)) (*Client, error) {
	// init defaults
	client := &Client{
		opts: clientOpts{
			OutputDirectory:   ".",
			InternalDirectory: "~/.local/share/doras",
			DockerConfigPath:  filepath.Join(os.Getenv("HOME"), ".docker", "config.json"),
		},
		backoff: backoff.DefaultBackoff(),
	}
	// apply provided opts
	for _, option := range options {
		option(client)
	}

	credFuncOpts := lo.Map(client.opts.CredFuncs, func(item auth.CredentialFunc, _ int) func(aggregate *ociutils.CredFuncAggregate) {
		return ociutils.WithCredFunc(item)
	})

	// load credentials from local docker cred store
	// this is done last so it does not shadow explicitly provided credentials
	store, err := credentials.NewStore(client.opts.DockerConfigPath, credentials.StoreOptions{
		DetectDefaultNativeStore: true,
	})
	if err != nil {
		log.Infof("failed to load credential store from %q", client.opts.DockerConfigPath)
	} else {
		credentialFunc := credentials.Credential(store)
		credFuncOpts = append(credFuncOpts, ociutils.WithCredFunc(credentialFunc))
	}

	// construct cred func that unifies all credential funcs
	credFunc := ociutils.NewCredentialsAggregate(credFuncOpts...)
	c, err := edgeapi.NewEdgeClient(client.opts.RemoteURL, client.opts.InsecureAllowHTTP, credFunc)
	if err != nil {
		return nil, err
	}
	client.edgeClient = c
	initialState := updaterstate.State{
		Version:        "1",
		ArtifactStates: make(map[string]string),
	}
	err = os.MkdirAll(client.opts.OutputDirectory, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	err = os.MkdirAll(client.opts.InternalDirectory, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create internal directory: %w", err)
	}
	client.patcherTmpDir = path.Join(client.opts.InternalDirectory, "patcher-dir")
	err = os.MkdirAll(client.patcherTmpDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create patcher working directory: %w", err)
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

	storageSource := fetcher.NewRepoStorageSource(false, credFunc)
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
func WithBackoffStrategy(b backoff.Strategy) func(*Client) {
	return func(c *Client) {
		c.backoff = b
	}
}

// WithCredential adds registry scoped credentials to the client.
func WithCredential(registry string, credential auth.Credential) func(*Client) {
	return func(c *Client) {
		c.opts.CredFuncs = append(c.opts.CredFuncs, auth.StaticCredential(registry, credential))
	}
}

// WithInsecureAllowHTTP configures the clients to allow HTTP requests.
func WithInsecureAllowHTTP(insecureAllowHTTP bool) func(*Client) {
	return func(c *Client) {
		c.opts.InsecureAllowHTTP = insecureAllowHTTP
	}
}

// WithKeepOldDir if set to true directories are not replaced, this is less robust.
func WithKeepOldDir(keepOldDir bool) func(*Client) {
	return func(c *Client) {
		c.opts.KeepOldDir = keepOldDir
	}
}
