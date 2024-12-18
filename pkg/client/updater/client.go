package updater

import (
	"github.com/unbasical/doras-server/pkg/client/edgeapi"
	"oras.land/oras-go/v2/registry/remote"
)

type dorasState struct {
	currentImage string
}

type Client struct {
	opts               clientOpts
	edgeClient         *edgeapi.Client
	acceptedAlgorithms []string
	state              dorasState
	deltaRepo          remote.Repository
}

func (c *Client) Pull(image string) error {
	for {
		exists, err := c.PullAsync(image)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
	}
}

// PullAsync Pull delta, but do not block if the delta has not been created yet.
// The result of the pull is according to the client configuration.
func (c *Client) PullAsync(target string) (exists bool, err error) {
	_, exists, err = c.edgeClient.ReadDelta(c.state.currentImage, target, c.acceptedAlgorithms)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	panic("not implemented")
}
