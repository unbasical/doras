package edgeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/unbasical/doras-server/internal/pkg/utils/buildurl"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras-server/internal/pkg/utils/ociutils"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/unbasical/doras-server/pkg/constants"

	log "github.com/sirupsen/logrus"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
)

func (c *Client) ReadDelta(from, to string, acceptedAlgorithms []string) (*apicommon.ReadDeltaResponse, bool, error) {
	log.Warnf("acceptedAlgorithms are not used: %s", acceptedAlgorithms)
	url := buildurl.New(
		buildurl.WithBasePath(c.base.DorasURL),
		buildurl.WithPathElement(apicommon.ApiBasePath),
		buildurl.WithPathElement(apicommon.DeltaApiPath),
		buildurl.WithQueryParam(constants.QueryKeyFromDigest, from),
		buildurl.WithQueryParam(constants.QueryKeyToTag, to),
	)

	resp, err := c.base.Client.Get(url)
	if err != nil {
		return nil, false, err
	}
	defer funcutils.PanicOrLogOnErr(resp.Body.Close, false, "failed to close response body")
	switch resp.StatusCode {
	case http.StatusOK:
		var resBody apicommon.ReadDeltaResponse
		err = json.NewDecoder(resp.Body).Decode(&resBody)
		if err != nil {
			return nil, false, err
		}
		return &resBody, true, nil
	case http.StatusAccepted:
		return nil, false, nil
	default:
		return nil, false, errors.New(resp.Status)
	}
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
	for {
		response, exists, err := c.ReadDelta(from, to, acceptedAlgorithms)
		if err != nil {
			return nil, "", nil, err
		}
		if exists {
			repoName, tag, _, err := apicommon.ParseOciImageString(response.DeltaImage)
			if err != nil {
				return nil, "", nil, err
			}
			repo, err := remote.NewRepository(repoName)
			if err != nil {
				return nil, "", nil, err
			}
			repo.Client = c.base.Client
			repo.PlainHTTP = true
			descriptor, rc, err := repo.FetchReference(context.Background(), tag)
			if err != nil {
				return nil, "", nil, err
			}
			defer funcutils.PanicOrLogOnErr(rc.Close, false, "failed to close fetch reader")
			var mf v1.Manifest
			err = json.NewDecoder(rc).Decode(&mf)
			if err != nil {
				return nil, "", nil, err
			}
			if len(mf.Layers) != 1 {
				return nil, "", nil, fmt.Errorf("unsupported number of layers %d", len(mf.Layers))
			}
			algo := strings.TrimPrefix(mf.Layers[0].MediaType, "application/")
			rc, err = repo.Blobs().Fetch(context.Background(), mf.Layers[0])
			if err != nil {
				return nil, "", nil, err
			}
			return &descriptor, algo, rc, nil
		}
		time.Sleep(time.Millisecond * 1000)
	}

}

func (c *Client) ReadDeltaAsDescriptor(from, to string, acceptedAlgorithms []string) (*v1.Descriptor, error) {
	log.Warnf("acceptedAlgorithms are not used: %s", acceptedAlgorithms)
	url := buildurl.New(
		buildurl.WithBasePath(c.base.DorasURL),
		buildurl.WithPathElement(apicommon.ApiBasePath),
		buildurl.WithPathElement(apicommon.DeltaApiPath),
		buildurl.WithQueryParam(constants.QueryKeyFromDigest, from),
		buildurl.WithQueryParam(constants.QueryKeyToTag, to),
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
	panic("not implemented")
	//return &resBody.DeltaDescriptor, nil
}
