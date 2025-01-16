package updater

import (
	"os"
	"path/filepath"

	"github.com/unbasical/doras-server/pkg/client/edgeapi"
)

type clientOpts struct {
	RemoteURL          string
	OutputDirectory    string
	InternalDirectory  string
	DockerConfigPath   string
	RegistryURL        string
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
	}

	for _, option := range options {
		option(client)
	}
	c, err := edgeapi.NewEdgeClient(client.opts.RemoteURL, client.opts.RegistryURL, true, nil)
	if err != nil {
		return nil, err
	}
	client.edgeClient = c
	client.state = &dorasState{fPath: client.opts.InternalDirectory}
	return client, nil
}

// WithRemoteURL adds the URL of a Doras server to the client configuration.
func WithRemoteURL(remoteURL string) func(*Client) {
	return func(c *Client) {
		c.opts.RemoteURL = remoteURL
	}
}

// WithRegistry adds a registry URL to the client configuration.
func WithRegistry(registry string) func(*Client) {
	return func(c *Client) {
		c.opts.RegistryURL = registry
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
