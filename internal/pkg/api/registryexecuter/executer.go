package registryexecuter

import (
	"context"
	"errors"
	"io"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/delta"
	dorasErrors "github.com/unbasical/doras-server/internal/pkg/error"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

type RegistryExecuter interface {
	ResolveAndLoad(target string, expectDigest bool) (v1.Descriptor, io.ReadCloser, error)
	Load(v1.Descriptor) (io.ReadCloser, error)
	CreateDummy(target string) error
	IsDummy(target string) (bool, error)
	PushDelta(target string, content io.Reader) error
}

type DeltaEngine struct {
	artifactStorageProvider apicommon.DorasStorage
	repoClients             map[string]remote.Client
}

func (e *DeltaEngine) ResolveAndLoad(target string, expectDigest bool) (v1.Descriptor, io.ReadCloser, error) {
	repo, tag, isDigest, err := apicommon.ParseOciImageString(target)
	if err != nil {
		return v1.Descriptor{}, nil, err
	}
	if expectDigest && !isDigest {
		return v1.Descriptor{}, nil, errors.New("expected digest")
	}
	source, err := e.getOrasSource(repo)
	if err != nil {
		return v1.Descriptor{}, nil, err
	}
	d, err := source.Resolve(context.Background(), tag)
	if err != nil {
		return v1.Descriptor{}, nil, err
	}
	rc, err := source.Fetch(context.Background(), d)
	if err != nil {
		return v1.Descriptor{}, nil, err
	}
	return d, rc, nil
}

func (e *DeltaEngine) CreateDummy(target string) error {
	//TODO implement me
	panic("implement me")
}

func (e *DeltaEngine) IsDummy(target string) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (e *DeltaEngine) PushDelta(target string, content io.Reader) error {
	//TODO implement me
	panic("implement me")
}

func NewDeltaEngine(artifactStorageProvider apicommon.DorasStorage, repoClients map[string]remote.Client) *DeltaEngine {
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
func (e *DeltaEngine) ReadDeltaImpl(from string, to string) (*apicommon.ReadDeltaResponse, error, string) {
	// Get oras targets and resolve the images into descriptors
	// TODO: consider parallelizing resolve with channels
	var srcFrom, srcTo oras.ReadOnlyTarget
	var descFrom, descTo v1.Descriptor
	for _, t := range []struct {
		t            *oras.ReadOnlyTarget
		i            string
		d            *v1.Descriptor
		mustBeDigest bool
	}{{&srcFrom, from, &descFrom, true}, {&srcTo, to, &descTo, false}} {
		repo, tag, isDigest, err := apicommon.ParseOciImageString(t.i)
		if err != nil {
			return nil, dorasErrors.ErrInternal, ""
		}
		// check for digest to make sure the request is not using a tagged image
		if !isDigest && t.mustBeDigest {
			return nil, dorasErrors.ErrBadRequest, "from image must be digest"
		}
		src, err := e.getOrasSource(repo)
		if err != nil {
			log.Errorf("Failed to get oras source: %s", err)
			return nil, dorasErrors.ErrInternal, ""
		}
		*t.t = src
		d, err := src.Resolve(context.Background(), tag)
		if err != nil {
			return nil, dorasErrors.ErrInternal, ""
		}
		*t.d = d
	}
	// Get an oras target for where we store the delta
	dst, err := e.artifactStorageProvider.GetStorage("deltas")
	if err != nil {
		return nil, dorasErrors.ErrInternal, ""
	}
	log.Warnf("currently always using the toImage registry as the source for fetches")
	log.Warn("currently not using the provided accepted algorithms")
	descDelta, err := delta.CreateDelta(context.Background(), srcTo, dst, descFrom, descTo)
	if err != nil {
		return nil, dorasErrors.ErrInternal, "failed to create delta"
	}
	return &apicommon.ReadDeltaResponse{Desc: *descDelta}, nil, ""
}
