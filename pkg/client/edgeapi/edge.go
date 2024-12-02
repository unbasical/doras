package edgeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/unbasical/doras-server/internal/pkg/utils/buildurl"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras-server/internal/pkg/utils/ociutils"
	"io"
	"net/http"

	"github.com/unbasical/doras-server/pkg/constants"

	log "github.com/sirupsen/logrus"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
)

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

func (c *Client) fetchManifestAndArtifact(target v1.Descriptor) (*v1.Manifest, io.ReadCloser, error) {
	// fetch manifest
	// fetch artifact via descriptor in manifest
	// consider using an oras storage to copy and then call fetch on local storage
	ctx := context.Background()
	repository, err := c.reg.Repository(ctx, "deltas")
	if err != nil {
		return nil, nil, err
	}
	rc, err := repository.Fetch(ctx, target)
	if err != nil {
		return nil, nil, err
	}
	defer funcutils.PanicOrLogOnErr(rc.Close, false, "failed to close fetch reader")
	var mf v1.Manifest
	err = json.NewDecoder(rc).Decode(&mf)
	if err != nil {
		return nil, nil, err
	}
	if len(mf.Layers) != 1 {
		return nil, nil, fmt.Errorf("unsupported number of layers %d", len(mf.Layers))
	}
	rc, err = repository.Blobs().Fetch(ctx, mf.Layers[0])
	if err != nil {
		return nil, nil, err
	}
	return &mf, rc, nil
}

func (c *Client) ReadDeltaAsStream(from, to string, acceptedAlgorithms []string) (*v1.Descriptor, string, io.ReadCloser, error) {
	descriptor, err := c.ReadDeltaAsDescriptor(from, to, acceptedAlgorithms)
	if err != nil {
		return nil, "", nil, err
	}
	mf, rc, err := c.fetchManifestAndArtifact(*descriptor)
	if err != nil {
		return nil, "", nil, err
	}
	algo, ok := mf.Annotations[constants.DorasAnnotationAlgorithm]
	if !ok {
		return nil, "", nil, fmt.Errorf("no algorithm found in manifest")
	}

	return descriptor, algo, rc, nil
}

func (c *Client) ReadDeltaAsDescriptor(from, to string, acceptedAlgorithms []string) (*v1.Descriptor, error) {
	log.Warnf("acceptedAlgorithms are not used: %s", acceptedAlgorithms)
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
	return &resBody.Desc, nil
}
