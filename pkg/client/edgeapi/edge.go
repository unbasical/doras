package edgeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/unbasical/doras-server/internal/pkg/client"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/buildurl"
	"github.com/unbasical/doras-server/internal/pkg/funcutils"
	"github.com/unbasical/doras-server/internal/pkg/ociutils"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
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

func (c *Client) LoadArtifact(artifactURL, outdir string) error {
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

func (c *Client) fetchArtifact(target v1.Descriptor) (io.ReadCloser, error) {
	// fetch manifest
	// fetch artifact via descriptor in manifest
	// consider using an oras storage to copy and then call fetch on local storage
	ctx := context.Background()
	repository, err := c.reg.Repository(ctx, "deltas")
	if err != nil {
		return nil, err
	}
	rc, err := repository.Fetch(ctx, target)
	if err != nil {
		return nil, err
	}
	defer funcutils.PanicOrLogOnErr(rc.Close, false, "failed to close fetch reader")
	var mf v1.Manifest
	err = json.NewDecoder(rc).Decode(&mf)
	if err != nil {
		return nil, err
	}
	if len(mf.Layers) != 1 {
		return nil, fmt.Errorf("unsupported number of layers %d", len(mf.Layers))
	}
	rc, err = repository.Blobs().Fetch(ctx, mf.Layers[0])
	if err != nil {
		return nil, err
	}
	return rc, nil
}

func (c *Client) ReadDelta(from, to string, acceptedAlgorithms []string) (io.ReadCloser, error) {

	url := buildurl.New(
		buildurl.WithBasePath(c.base.DorasURL),
		buildurl.WithPathElement(apicommon.ApiBasePath),
		buildurl.WithPathElement(apicommon.DeltaApiPath),
		buildurl.WithQueryParam("from", from),
		buildurl.WithQueryParam("to", to),
	)

	resp, err := c.base.Client.Get(url)
	if err != nil {
		return nil, err
	}
	defer funcutils.PanicOrLogOnErr(resp.Body.Close, false, "failed to close response body")
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unpexpected status code: %s", resp.Status)
	}
	var resBody apicommon.ReadDeltaResponse
	err = json.NewDecoder(resp.Body).Decode(&resBody)
	if err != nil {
		return nil, err
	}
	return c.fetchArtifact(resBody.Desc)
}
