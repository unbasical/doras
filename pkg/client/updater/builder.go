package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	"github.com/unbasical/doras/pkg/backoff"
	"github.com/unbasical/doras/pkg/client/updater/fetcher"
	"github.com/unbasical/doras/pkg/client/updater/inspector"
	"github.com/unbasical/doras/pkg/client/updater/statemanager"
	"github.com/unbasical/doras/pkg/client/updater/updaterstate"
	"github.com/unbasical/doras/pkg/client/updater/validator"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"

	"github.com/unbasical/doras/pkg/client/edgeapi"
)

type clientOpts struct {
	RemoteURL            string
	OutputDirectory      string
	OutputDirPermissions os.FileMode
	InternalDirectory    string
	DockerConfigPath     string
	AcceptedAlgorithms   []string
	CredFuncs            []auth.CredentialFunc
	InsecureAllowHTTP    bool
	KeepOldDir           bool
	Validators           []validator.ManifestValidator
	Inspectors           []inspector.ArtifactInspector
}

// NewClient creates a new Doras update client with the provided options.
//
//nolint:revive
func NewClient(options ...func(*Client)) (*Client, error) {
	// init defaults
	client := &Client{
		opts: clientOpts{
			OutputDirectory:      ".",
			InternalDirectory:    "~/.local/share/doras",
			DockerConfigPath:     filepath.Join(os.Getenv("HOME"), ".docker", "config.json"),
			OutputDirPermissions: 0777,
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
		Version:        "2",
		ArtifactStates: make(map[string]updaterstate.ArtifactState),
	}

	statePath := path.Join(client.opts.InternalDirectory, "doras-state.json")
	// detect old state files and remove them
	// this is purely for compatability reasons
	err = cleanupLegacyState(err, statePath, client)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(client.opts.OutputDirectory)
	if errors.Is(err, os.ErrNotExist) {
		log.Infof(
			"creating output-directory at: %q with permissions: %o",
			client.opts.OutputDirectory,
			client.opts.OutputDirPermissions,
		)
	}
	err = os.MkdirAll(client.opts.OutputDirectory, client.opts.OutputDirPermissions)
	if err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	stat, err = os.Stat(client.opts.OutputDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to stat output directory: %w", err)
	}
	if stat.Mode() != client.opts.OutputDirPermissions|os.ModeDir {
		log.Warn("got output directory permissions that do not match expected, running chmod")
		err := os.Chmod(client.opts.OutputDirectory, client.opts.OutputDirPermissions)
		if err != nil {
			return nil, fmt.Errorf("failed to chmod output directory: %w", err)
		}
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

	stateManager, err := statemanager.NewFromDisk(initialState, statePath)
	if err != nil {
		return nil, err
	}
	// make sure state is written to the file in case it was not initialized
	err = stateManager.Commit()
	if err != nil {
		return nil, err
	}
	client.state = stateManager
	fetcherDir := path.Join(client.opts.InternalDirectory, "fetcher")
	err = os.Mkdir(fetcherDir, client.opts.OutputDirPermissions)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}

	storageSource := fetcher.NewRepoStorageSource(false, credFunc)
	client.reg = fetcher.NewArtifactLoader(fetcherDir, storageSource, client.opts.Validators, client.opts.Inspectors)
	return client, nil
}

func cleanupLegacyState(err error, statePath string, client *Client) error {
	oldState, err := os.ReadFile(statePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to read old state: %w", err)
	}
	if !errors.Is(err, os.ErrNotExist) {
		s := struct {
			Version        string         `json:"version"`
			ArtifactStates map[string]any `json:"artifact_states"`
		}{}
		err = json.Unmarshal(oldState, &s)
		if err != nil {
			log.Infof("purging internal directory at %v (failed to unmarshal old state: %v)", client.opts.InternalDirectory, err)
			err := os.RemoveAll(client.opts.InternalDirectory)
			if err != nil {
				return fmt.Errorf("failed to remove old state: %w", err)
			}
			return nil
		}
		if s.Version == "1" {
			log.Infof("Detected version 1 state file, deleting for sanity reasons")
			err := os.RemoveAll(client.opts.InternalDirectory)
			if err != nil {
				return fmt.Errorf("failed to remove old state: %w", err)
			}
		}
	}
	return nil
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

// WithOutputDirPermissions allows users to control the permissions of the output directory.
func WithOutputDirPermissions(outputDirPermissions os.FileMode) func(*Client) {
	return func(c *Client) {
		c.opts.OutputDirPermissions = outputDirPermissions
	}
}

// WithInternalDirectory sets the client configurations local working directory.
// It stores things such as the updater's internal state.
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

// WithManifestValidators adds a collection of validators to the client which inspect the manifest before fetching the artifact.
func WithManifestValidators(validators []validator.ManifestValidator) func(client *Client) {
	return func(c *Client) {
		c.opts.Validators = validators
	}
}

// WithArtifactInspectors configures a Client with a list of ArtifactInspector instances to be used for artifact inspection.
func WithArtifactInspectors(inspectors []inspector.ArtifactInspector) func(client *Client) {
	return func(c *Client) {
		c.opts.Inspectors = inspectors
	}
}
