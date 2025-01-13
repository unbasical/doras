package updater

import (
	"os"
	"path/filepath"

	"github.com/unbasical/doras-server/pkg/client/edgeapi"
)

type clientOpts struct {
	RemoteURL         string
	OutputDirectory   string
	InternalDirectory string
	DockerConfigPath  string
	RegistryURL       string
}

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
	return client, nil
}

func WithRemoteURL(remoteURL string) func(*Client) {
	return func(c *Client) {
		c.opts.RemoteURL = remoteURL
	}
}
func WithRegistry(registry string) func(*Client) {
	return func(c *Client) {
		c.opts.RegistryURL = registry
	}
}
func WithOutputDirectory(outputDirectory string) func(*Client) {
	return func(c *Client) {
		c.opts.OutputDirectory = outputDirectory
	}
}

func WithInternalDirectory(internalDirectory string) func(*Client) {
	return func(c *Client) {
		c.opts.InternalDirectory = internalDirectory
	}
}

func WithDockerConfigPath(dockerConfigPath string) func(*Client) {
	return func(c *Client) {
		c.opts.DockerConfigPath = dockerConfigPath
	}
}
