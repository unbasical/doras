package updater

import "github.com/unbasical/doras-server/pkg/client/edgeapi"

type Client struct {
	opts               clientOpts
	edgeClient         *edgeapi.Client
	currentVersion     string
	acceptedAlgorithms []string
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
func (c *Client) PullAsync(target string) (exists bool, bool error) {
	panic("todo")
	// TODO:
	// 1. Check if target matches current version, return (true, nil) if yes.
	// 2. Request delta, return (false, nil) if the delta was accepted but has not been created yet.
	// 3. Pull delta and apply. Return (true, nil) if the delta has been applied successfully.
}
