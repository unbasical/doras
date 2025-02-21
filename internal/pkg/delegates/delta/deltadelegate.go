package deltadelegate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"

	registrydelegate "github.com/unbasical/doras/internal/pkg/delegates/registry"

	"github.com/opencontainers/go-digest"
	"github.com/unbasical/doras/pkg/constants"
)

type delegate struct {
	activeRequests          map[string]any
	m                       sync.Mutex
	dummyExpirationDuration time.Duration
}

// NewDeltaDelegate construct a DeltaDelegate that is used to handle delta creation operations.
func NewDeltaDelegate(dummyExpirationDuration time.Duration) DeltaDelegate {
	return &delegate{
		activeRequests:          make(map[string]any),
		m:                       sync.Mutex{},
		dummyExpirationDuration: dummyExpirationDuration,
	}
}

func (d *delegate) IsDummy(mf ociutils.Manifest) (isDummy bool, expired bool) {
	if mf.Annotations[constants.DorasAnnotationIsDummy] != "true" {
		return false, false
	}
	isDummy = true
	ts := mf.Annotations["org.opencontainers.image.created"]
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return false, false
	}
	expiration := t.Add(d.dummyExpirationDuration)
	now := time.Now()
	expired = now.After(expiration)
	return
}

func (d *delegate) GetDeltaLocation(deltaMf registrydelegate.DeltaManifestOptions) (string, error) {
	digestFrom, err := extractDigest(deltaMf.From)
	if err != nil {
		return "", err
	}
	digestTo, err := extractDigest(deltaMf.To)
	if err != nil {
		return "", err
	}
	dgstIdentifier := digest.FromBytes([]byte(digestFrom.Encoded() + digestTo.Encoded() + deltaMf.GetTagSuffix()))
	repoName, _, _, err := ociutils.ParseOciImageString(deltaMf.From)
	if err != nil {
		return "", err
	}
	tag := fmt.Sprintf("_delta-%s", dgstIdentifier.Encoded())
	image := fmt.Sprintf("%s:%s", repoName, tag)
	return image, nil
}

func extractDigest(image string) (*digest.Digest, error) {
	_, tag, isDigest, err := ociutils.ParseOciImageString(image)
	if err != nil {
		return nil, err
	}
	if !isDigest {
		return nil, errors.New("expected image with digest")
	}
	dgst := strings.TrimPrefix(tag, "@sha256:")
	val := digest.NewDigestFromEncoded("sha256", dgst)
	return &val, nil
}

func (d *delegate) CreateDelta(ctx context.Context, from, to io.ReadCloser, manifOpts registrydelegate.DeltaManifestOptions, dst registrydelegate.RegistryDelegate) error {
	deltaReader, err := manifOpts.Differ.Diff(from, to)
	if err != nil {
		return err
	}
	compressedDelta, err := manifOpts.Compressor.Compress(deltaReader)
	if err != nil {
		return err
	}
	deltaLocationWithTag, err := d.GetDeltaLocation(manifOpts)
	if err != nil {
		return err
	}
	deltaId := digest.FromBytes([]byte(deltaLocationWithTag))
	log.WithField("delta-id", deltaId).Info("delta creation started")
	d.m.Lock()
	if _, ok := d.activeRequests[deltaLocationWithTag]; ok {
		return nil
	}
	d.activeRequests[deltaLocationWithTag] = nil
	d.m.Unlock()
	log.Debugf("handling request for %q", deltaLocationWithTag)
	err = dst.PushDelta(ctx, deltaLocationWithTag, manifOpts, compressedDelta)
	d.m.Lock()
	delete(d.activeRequests, deltaLocationWithTag)
	d.m.Unlock()
	if err != nil {
		return err
	}
	return nil
}

// DeltaDelegate abstracts over operations that are required to create deltas.
type DeltaDelegate interface {
	// IsDummy checks if the provided ociutils.Manifest refers to a dummy image that has not expired.
	// Dummy images are used to communicate to other Doras instances that a server is working on creating this delta.
	// This method should handle synchronization at the instance level.
	IsDummy(mf ociutils.Manifest) (isDummy bool, expired bool)
	// GetDeltaLocation returns the image at which the delta with the given options is/should be stored.
	GetDeltaLocation(deltaMf registrydelegate.DeltaManifestOptions) (string, error)
	// CreateDelta constructs the delta and pushes it to the registry.
	// This method should handle synchronization at the instance level.
	CreateDelta(ctx context.Context, from, to io.ReadCloser, manifOpts registrydelegate.DeltaManifestOptions, dst registrydelegate.RegistryDelegate) error
}
