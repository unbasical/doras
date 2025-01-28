package edgeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/unbasical/doras/internal/pkg/auth"
	"io"
	"math/rand"
	"net/http"
	auth2 "oras.land/oras-go/v2/registry/remote/auth"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras/internal/pkg/api/apicommon"
	"github.com/unbasical/doras/internal/pkg/client"
	"github.com/unbasical/doras/internal/pkg/utils/buildurl"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras/pkg/constants"
	"oras.land/oras-go/v2/registry/remote"
)

// Client provides the functionality to interact with the Doras server API.
type Client struct {
	base      *client.DorasBaseClient
	backoff   BackoffStrategy
	plainHTTP bool
}

// exponentialBackoffWithJitter implements the BackoffStrategy interface
type exponentialBackoffWithJitter struct {
	baseDelay      time.Duration // Base delay between retries (e.g., 100ms)
	maxDelay       time.Duration // Maximum delay before giving up
	currentAttempt uint          // Track the current attempt number
	maxAttempt     uint          // Track the current attempt number
	randSource     *rand.Rand    // Random source for jittering
}

// NewExponentialBackoffWithJitter creates a new instance of exponentialBackoffWithJitter
func NewExponentialBackoffWithJitter(baseDelay, maxDelay time.Duration, maxAttempts uint) BackoffStrategy {
	// Seed the random number generator for jitter
	source := rand.NewSource(time.Now().UnixNano())
	return &exponentialBackoffWithJitter{
		baseDelay:      baseDelay,
		maxDelay:       maxDelay,
		currentAttempt: 0,
		maxAttempt:     maxAttempts,
		randSource:     rand.New(source),
	}
}

// Wait calculates the next backoff time with exponential backoff and jitter
func (e *exponentialBackoffWithJitter) Wait() error {
	if e.currentAttempt >= e.maxAttempt {
		return errors.New("maximum retries exceeded")
	}
	// Calculate the exponential backoff delay
	delay := e.baseDelay * time.Duration(1<<e.currentAttempt) // 2^attempt * baseDelay

	// Apply jitter by adding a random factor to the delay (between 0 and 1x the delay)
	jitter := time.Duration(e.randSource.Int63n(int64(delay)))
	delay = delay + jitter - (delay / 2) // Apply jitter in both directions

	// Ensure that delay does not exceed the maximum delay
	if delay > e.maxDelay {
		delay = e.maxDelay
	}

	// Sleep for the calculated delay
	log.Debugf("Waiting for %v (attempt %d)\n", delay, e.currentAttempt)
	time.Sleep(delay)

	// Increment the attempt number for the next retry
	e.currentAttempt++
	return nil
}

// BackoffStrategy is used to avoid flooding the server with requests
// when clients are waiting for the delta request to be completed.
type BackoffStrategy interface {
	Wait() error
}

// DefaultBackoff returns a sensible default BackoffStrategy (exponential with an upper bound).
func DefaultBackoff() BackoffStrategy {
	const defaultBaseDelay = 100 * time.Millisecond
	const defaultMaxDelay = 1 * time.Minute
	return NewExponentialBackoffWithJitter(defaultBaseDelay, defaultMaxDelay, 10)
}

// NewEdgeClient returns a client that can be used to interact with the Doras server API.
func NewEdgeClient(serverURL string, allowHttp bool, tokenProvider client.AuthProvider) (*Client, error) {
	//if tokenProvider != nil && allowHttp {
	//	return nil, errors.New("using a login token while allowing HTTP is not supported to avoid leaking credentials")
	//}
	return &Client{
		base:      client.NewBaseClient(serverURL, tokenProvider),
		backoff:   DefaultBackoff(),
		plainHTTP: allowHttp,
	}, nil
}

// ReadDeltaAsync requests a delta between the two provided images and returns the server's response.
// The function does not block if the delta is still being created.
// If the delta has been created exists will be set to true.
// If `err == nil && exists` is true then the request has been accepted by the server but the delta has not been created.
func (c *Client) ReadDeltaAsync(from, to string, acceptedAlgorithms []string) (res *apicommon.ReadDeltaResponse, exists bool, err error) {
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

	if c.base.TokenProvider != nil {
		log.Debug("attempting to load token")
		creds, err := c.base.TokenProvider.GetAuth()
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
func (c *Client) ReadDelta(from, to string, acceptedAlgorithms []string) (*apicommon.ReadDeltaResponse, error) {
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
func (c *Client) ReadDeltaAsStream(from, to string, acceptedAlgorithms []string) (*v1.Descriptor, string, io.ReadCloser, error) {
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
