package edgeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/unbasical/doras-server/internal/pkg/utils/ociutils"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
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

// exponentialBackoffWithJitter implements the BackoffStrategy interface
type exponentialBackoffWithJitter struct {
	baseDelay      time.Duration // Base delay between retries (e.g., 100ms)
	maxDelay       time.Duration // Maximum delay before giving up
	currentAttempt uint          // Track the current attempt number
	maxAttempt     uint          // Track the current attempt number
	randSource     *rand.Rand    // Random source for jittering
}

// NewExponentialBackoffWithJitter creates a new instance of exponentialBackoffWithJitter
func NewExponentialBackoffWithJitter(baseDelay, maxDelay time.Duration, maxAttempts uint) *exponentialBackoffWithJitter {
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
	fmt.Printf("Waiting for %v (attempt %d)\n", delay, e.currentAttempt)
	time.Sleep(delay)

	// Increment the attempt number for the next retry
	e.currentAttempt++
	return nil
}

type BackoffStrategy interface {
	Wait() error
}

func DefaultBackoff() BackoffStrategy {
	return NewExponentialBackoffWithJitter(1000*time.Millisecond, 1*time.Minute, 5)
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
	repoName, tag, _, err := ociutils.ParseOciImageString(response.DeltaImage)
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
