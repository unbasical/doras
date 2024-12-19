package deltadelegate

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	registrydelegate "github.com/unbasical/doras-server/internal/pkg/delegates/registry"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/pkg/constants"
)

type Delegate struct {
	baseUrl        string
	activeRequests map[string]any
	m              sync.Mutex
}

func NewDeltaDelegate(baseUrl string) DeltaDelegate {
	return &Delegate{
		baseUrl:        baseUrl,
		activeRequests: make(map[string]any),
		m:              sync.Mutex{},
	}
}

func (d *Delegate) IsDummy(mf v1.Manifest) (isDummy bool, expired bool) {
	if mf.Annotations[constants.DorasAnnotationIsDummy] != "true" {
		return false, false
	}
	isDummy = true
	ts := mf.Annotations["org.opencontainers.image.created"]
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return false, false
	}
	expiration := t.Add(5 * time.Minute)
	now := time.Now()
	expired = now.After(expiration)
	return
}

func (d *Delegate) GetDeltaLocation(deltaMf registrydelegate.DeltaManifestOptions) (string, error) {
	digestFrom, err := extractDigest(deltaMf.From)
	if err != nil {
		return "", err
	}
	digestTo, err := extractDigest(deltaMf.To)
	if err != nil {
		return "", err
	}
	repoName := fmt.Sprintf("%s/%s/%s", d.baseUrl, digestFrom.Encoded(), digestTo.Encoded())
	return repoName, nil
}

func extractDigest(image string) (*digest.Digest, error) {
	_, tag, isDigest, err := apicommon.ParseOciImageString(image)
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

func (d *Delegate) CreateDelta(from, to io.ReadCloser, manifOpts registrydelegate.DeltaManifestOptions, dst registrydelegate.RegistryDelegate) error {
	// TODO: avoid leaking readers
	deltaReader, err := manifOpts.Differ.Diff(from, to)
	if err != nil {
		return err
	}
	compressedDelta, err := manifOpts.Compressor.Compress(deltaReader)
	if err != nil {
		return err
	}
	deltaLocation, err := d.GetDeltaLocation(manifOpts)
	if err != nil {
		return err
	}
	deltaLocationWithTag := fmt.Sprintf("%s:%s", deltaLocation, manifOpts.GetTag())
	d.m.Lock()
	if _, ok := d.activeRequests[deltaLocationWithTag]; ok {
		return nil
	}
	d.activeRequests[deltaLocationWithTag] = nil
	d.m.Unlock()
	err = dst.PushDelta(deltaLocationWithTag, manifOpts, compressedDelta)
	d.m.Lock()
	delete(d.activeRequests, deltaLocationWithTag)
	d.m.Unlock()
	if err != nil {
		return err
	}
	return nil
}

type DeltaDelegate interface {
	IsDummy(mf v1.Manifest) (isDummy bool, expired bool)
	GetDeltaLocation(deltaMf registrydelegate.DeltaManifestOptions) (string, error)
	CreateDelta(from, to io.ReadCloser, manifOpts registrydelegate.DeltaManifestOptions, dst registrydelegate.RegistryDelegate) error
}
