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

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/client"
	"github.com/unbasical/doras-server/internal/pkg/utils/buildurl"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras-server/pkg/constants"
	"oras.land/oras-go/v2/registry/remote"
)

type Client struct {
	base    *client.DorasBaseClient
	reg     *remote.Registry
	backoff BackoffStrategy
}

type BackoffStrategy interface {
	Wait() error
}

type fixedBackoff struct {
	interval     time.Duration
	attemptsLeft uint
}

func DefaultBackoff() BackoffStrategy {
	return &fixedBackoff{
		interval:     100 * time.Millisecond,
		attemptsLeft: 5,
	}
}

func (f *fixedBackoff) Wait() error {
	if f.attemptsLeft == 0 {
		return errors.New("backoff exceeded attempts limit")
	}
	f.attemptsLeft--
	time.Sleep(f.interval)
	return nil
}

func NewEdgeClient(serverURL, registry string, allowHttp bool) (*Client, error) {
	reg, err := remote.NewRegistry(registry)
	if err != nil {
		return nil, err
	}
	reg.PlainHTTP = allowHttp
	return &Client{
		base:    client.NewBaseClient(serverURL),
		reg:     reg,
		backoff: DefaultBackoff(),
	}, nil
}

func (c *Client) ReadDeltaAsync(from, to string, acceptedAlgorithms []string) (*apicommon.ReadDeltaResponse, bool, error) {
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

func (c *Client) ReadDelta(from, to string, acceptedAlgorithms []string) (*apicommon.ReadDeltaResponse, error) {
	for {
		response, exists, err := c.ReadDeltaAsync(from, to, acceptedAlgorithms)
		if err != nil {
			return nil, err
		}
		if exists {
			return response, nil
		}
		err = c.backoff.Wait()
		if err != nil {
			return nil, err
		}
	}
}

func (c *Client) ReadDeltaAsStream(from, to string, acceptedAlgorithms []string) (*v1.Descriptor, string, io.ReadCloser, error) {
	response, err := c.ReadDelta(from, to, acceptedAlgorithms)
	if err != nil {
		return nil, "", nil, err
	}
	repoName, tag, _, err := apicommon.ParseOciImageString(response.DeltaImage)
	if err != nil {
		return nil, "", nil, err
	}
	repo, err := remote.NewRepository(repoName)
	if err != nil {
		return nil, "", nil, err
	}
	repo.Client = c.base.Client
	repo.PlainHTTP = c.reg.PlainHTTP
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
