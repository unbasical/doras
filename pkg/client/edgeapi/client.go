package edgeapi

import (
	"github.com/unbasical/doras-server/internal/pkg/client"
	"oras.land/oras-go/v2/registry/remote"
)

type Client struct {
	base *client.DorasBaseClient
	reg  *remote.Registry
}

func NewEdgeClient(serverURL, registry string, allowHttp bool) (*Client, error) {
	reg, err := remote.NewRegistry(registry)
	if err != nil {
		return nil, err
	}
	reg.PlainHTTP = allowHttp
	return &Client{
		base: client.NewBaseClient(serverURL),
		reg:  reg,
	}, nil
}
