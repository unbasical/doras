package cloudapi

import "github.com/unbasical/doras-server/internal/pkg/client"

type Client struct {
	base *client.DorasBaseClient
}

func NewClient(serverURL string) *Client {
	return &Client{base: client.NewBaseClient(serverURL)}
}
