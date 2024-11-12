package client

import (
	"context"

	"github.com/unbasical/doras-server/internal/pkg/ociutils"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
)

type EdgeClient struct {
	base *dorasBaseClient
	reg  *remote.Registry
}

func NewEdgeClient(serverURL, registry string, allowHttp bool) (*EdgeClient, error) {
	reg, err := remote.NewRegistry(registry)
	if err != nil {
		return nil, err
	}
	reg.PlainHTTP = allowHttp
	return &EdgeClient{
		base: newBaseClient(serverURL),
		reg:  reg,
	}, nil
}

func (c *EdgeClient) LoadArtifact(artifactURL, outdir string) error {
	identifier, err := ociutils.NewImageIdentifier(artifactURL)
	if err != nil {
		return err
	}
	s, err := file.New(outdir)
	if err != nil {
		return err
	}
	repo, err := c.reg.Repository(context.Background(), identifier.Repository())
	if err != nil {
		return err
	}
	_, err = oras.Copy(context.Background(), repo, identifier.TagOrDigest(), s, identifier.TagOrDigest(), oras.DefaultCopyOptions)
	if err != nil {
		return err
	}
	return nil
}
