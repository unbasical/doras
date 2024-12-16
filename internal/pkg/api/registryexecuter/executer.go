package registryexecuter

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/unbasical/doras-server/pkg/compression"
	delta2 "github.com/unbasical/doras-server/pkg/delta"

	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/delta"
	dorasErrors "github.com/unbasical/doras-server/internal/pkg/error"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

type RegistryExecuter interface {
	ResolveAndLoadManifest(target string, expectDigest bool) (v1.Descriptor, v1.Manifest, error)
	Load(v1.Descriptor) (io.ReadCloser, error)
	CreateDummy(mfOpts DeltaManifestOptions) error
	IsDummy(mf v1.Manifest) (bool, error)
	PushDelta(target string, content io.Reader) error
	ResolveDelta(from v1.Descriptor, to v1.Descriptor) (v1.Descriptor, error)
}

type DeltaEngine struct {
	artifactStorageProvider apicommon.ArtifactStorage
	deltaStorageProvider    apicommon.DeltaStorage
	repoClients             map[string]remote.Client
}

func (e *DeltaEngine) Load(descriptor v1.Descriptor) (io.ReadCloser, error) {
	//TODO implement me
	panic("implement me")
}

func (e *DeltaEngine) CreateDummy(mfOpts DeltaManifestOptions) error {
	//TODO implement me
	return errors.New("not implemented")
}

func (e *DeltaEngine) IsDummy(mf v1.Manifest) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (e *DeltaEngine) ResolveAndLoadManifest(target string, expectDigest bool) (v1.Descriptor, v1.Manifest, oras.ReadOnlyTarget, error) {
	repo, tag, isDigest, err := apicommon.ParseOciImageString(target)
	if err != nil {
		return v1.Descriptor{}, v1.Manifest{}, nil, err
	}
	if expectDigest && !isDigest {
		return v1.Descriptor{}, v1.Manifest{}, nil, errors.New("expected digest")
	}
	source, err := e.getOrasSource(repo)
	if err != nil {
		return v1.Descriptor{}, v1.Manifest{}, nil, err
	}
	d, err := source.Resolve(context.Background(), tag)
	if err != nil {
		return v1.Descriptor{}, v1.Manifest{}, nil, err
	}
	rc, err := source.Fetch(context.Background(), d)
	if err != nil {
		return v1.Descriptor{}, v1.Manifest{}, nil, err
	}
	defer funcutils.PanicOrLogOnErr(rc.Close, false, "failed to close reader")

	mf := v1.Manifest{}
	err = json.NewDecoder(rc).Decode(&mf)
	if err != nil {
		return v1.Descriptor{}, v1.Manifest{}, nil, err
	}
	return d, mf, source, nil
}

func (e *DeltaEngine) PushDelta(target string, content io.Reader) error {
	//TODO implement me
	panic("implement me")
}

func (e *DeltaEngine) ResolveDelta(from v1.Descriptor, to v1.Descriptor, tag string) (v1.Manifest, error) {
	_, target, err := e.deltaStorageProvider.GetDeltaStorage(from, to)
	if err != nil {
		return v1.Manifest{}, err
	}
	d, err := target.Resolve(context.Background(), tag)
	if err != nil {
		return v1.Manifest{}, err
	}
	rc, err := target.Fetch(context.Background(), d)
	if err != nil {
		return v1.Manifest{}, err
	}
	defer funcutils.PanicOrLogOnErr(rc.Close, false, "failed to close reader")
	mf := v1.Manifest{}
	err = json.NewDecoder(rc).Decode(&mf)
	if err != nil {
		return v1.Manifest{}, err
	}
	return mf, nil
}

func NewDeltaEngine(artifactStorageProvider apicommon.ArtifactStorage, repoClients map[string]remote.Client) *DeltaEngine {
	return &DeltaEngine{
		artifactStorageProvider: artifactStorageProvider,
		repoClients:             repoClients,
	}
}

func (e *DeltaEngine) getOrasSource(repoUrl string) (oras.ReadOnlyTarget, error) {
	src, err := remote.NewRepository(repoUrl)
	if err != nil {
		return nil, err
	}
	src.PlainHTTP = true
	if c, ok := e.repoClients[repoUrl]; ok {
		src.Client = c
	} else {
		log.Debugf("did not find client configuration for %s, using default config", repoUrl)
	}
	return src, nil
}
func (e *DeltaEngine) ReadDeltaImpl(source oras.ReadOnlyTarget, from, to v1.Descriptor) (*v1.Descriptor, error, string) {

	// Get an oras target for where we store the delta
	_, dst, err := e.artifactStorageProvider.GetStorage("deltas")
	if err != nil {
		return nil, dorasErrors.ErrInternal, ""
	}
	log.Warnf("currently always using the toImage registry as the source for fetches")
	log.Warn("currently not using the provided accepted algorithms")
	descDelta, err := delta.CreateDelta(context.Background(), source, dst, from, to)
	if err != nil {
		return nil, dorasErrors.ErrInternal, "failed to create delta"
	}
	return descDelta, nil, ""
}

type DeltaManifestOptions struct {
	From string
	To   string
	AlgorithmChoice
}

type AlgorithmChoice struct {
	delta2.Differ
	compression.Compressor
}

func (c *AlgorithmChoice) GetTag() string {
	if compressorName := c.Compressor.Name(); compressorName != "" {
		return c.Differ.Name() + "_" + compressorName
	}
	return c.Differ.Name()
}
