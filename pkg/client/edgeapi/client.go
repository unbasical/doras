package edgeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/auth"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"
	backoff2 "github.com/unbasical/doras/pkg/backoff"
	"io"
	"net/http"
	auth2 "oras.land/oras-go/v2/registry/remote/auth"
	"strings"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras/internal/pkg/api/apicommon"
	"github.com/unbasical/doras/internal/pkg/client"
	"github.com/unbasical/doras/internal/pkg/utils/buildurl"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras/pkg/constants"
	"oras.land/oras-go/v2/registry/remote"
)

// deltaApiClient provides the functionality to interact with the Doras server API.
type deltaApiClient struct {
	base      *client.DorasBaseClient
	backoff   backoff2.Strategy
	plainHTTP bool
}

// NewEdgeClient returns a client that can be used to interact with the Doras server API.
func NewEdgeClient(serverURL string, allowHttp bool, credentialFunc auth2.CredentialFunc) (DeltaApiClient, error) {
	//if tokenProvider != nil && allowHttp {
	//	return nil, errors.New("using a login token while allowing HTTP is not supported to avoid leaking credentials")
	//}
	return &deltaApiClient{
		base:      client.NewBaseClient(serverURL, credentialFunc),
		backoff:   backoff2.DefaultBackoff(),
		plainHTTP: allowHttp,
	}, nil
}

// ReadDeltaAsync requests a delta between the two provided images and returns the server's response.
// The function does not block if the delta is still being created.
// If the delta has been created exists will be set to true.
// If `err == nil && exists` is true then the request has been accepted by the server but the delta has not been created.
func (c *deltaApiClient) ReadDeltaAsync(from, to string, acceptedAlgorithms []string) (res *apicommon.ReadDeltaResponse, exists bool, err error) {
	url := buildurl.New(
		buildurl.WithBasePath(c.base.DorasURL),
		buildurl.WithPathElement(apicommon.ApiBasePathV1),
		buildurl.WithPathElement(apicommon.DeltaApiPath),
		buildurl.WithQueryParam(constants.QueryKeyFromDigest, from),
		buildurl.WithQueryParam(constants.QueryKeyToTag, to),
		buildurl.WithListQueryParam(constants.QueryKeyAcceptedAlgorithm, acceptedAlgorithms),
	)

	log.Debugf("sending delta request to %s", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, false, err
	}

	if c.base.CredentialFunc != nil {
		log.Debug("attempting to load token")
		ociUrl, err := ociutils.ParseOciUrl(from)
		if err != nil {
			return nil, false, err
		}
		creds, err := c.base.CredentialFunc(context.Background(), ociUrl.Host)
		if err != nil {
			log.WithError(err).Debug("could not load auth token, using no authentication")
		} else {
			setupAuthHeader(creds, req)
		}
	}
	resp, err := c.base.Client.Do(req)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Error(err)
		}
	}()
	switch resp.StatusCode {
	case http.StatusOK:
		var resBody apicommon.ReadDeltaResponse
		decoder := json.NewDecoder(resp.Body)
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&resBody)
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

func setupAuthHeader(creds auth2.Credential, req *http.Request) {
	if creds.AccessToken != "" {
		log.Debug("using an access token")
		req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
		return
	}
	if creds.Username != "" && creds.Password != "" {
		log.Debug("using basic auth")
		req.Header.Set("Authorization", auth.GenerateBasicAuth(creds.Username, creds.Password))
		return
	}
	log.Debug("no header was set")
}

// ReadDelta requests a delta between the two provided images and returns the server's response.
// Blocks until the delta has been created or an error is detected.
// The server supports non-blocking requests for deltas, to use them use the sibling function ReadDeltaAsync.
func (c *deltaApiClient) ReadDelta(from, to string, acceptedAlgorithms []string) (*apicommon.ReadDeltaResponse, error) {
	for {
		response, exists, err := c.ReadDeltaAsync(from, to, acceptedAlgorithms)
		if err != nil {
			return nil, err
		}
		if exists {
			return response, nil
		}
		log.Debugf("request was accepted, trying again after waiting period")
		err = c.backoff.Wait()
		if err != nil {
			return nil, err
		}
	}
}

// ReadDeltaAsStream requests a delta between the two provided images and reads it as a stream.
func (c *deltaApiClient) ReadDeltaAsStream(from, to string, acceptedAlgorithms []string) (*v1.Descriptor, string, io.ReadCloser, error) {
	response, err := c.ReadDelta(from, to, acceptedAlgorithms)
	if err != nil {
		return nil, "", nil, err
	}
	repoName, tag, _, err := ociutils.ParseOciImageString(response.DeltaImage)
	if err != nil {
		return nil, "", nil, err
	}
	repo, err := remote.NewRepository(repoName)
	if err != nil {
		return nil, "", nil, err
	}
	repo.Client = c.base.Client
	repo.PlainHTTP = c.plainHTTP
	descriptor, rc, err := repo.FetchReference(context.Background(), tag)
	if err != nil {
		return nil, "", nil, err
	}
	defer funcutils.PanicOrLogOnErr(rc.Close, false, "failed to close fetch reader")
	var mf ociutils.Manifest
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

// DeltaApiClient abstracts around a client that can request deltas from Doras servers.
type DeltaApiClient interface {
	ReadDeltaAsync(from, to string, acceptedAlgorithms []string) (res *apicommon.ReadDeltaResponse, exists bool, err error)
	ReadDelta(from, to string, acceptedAlgorithms []string) (*apicommon.ReadDeltaResponse, error)
	ReadDeltaAsStream(from, to string, acceptedAlgorithms []string) (*v1.Descriptor, string, io.ReadCloser, error)
}
