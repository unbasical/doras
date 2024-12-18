package registrydelegate

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/unbasical/doras-server/internal/pkg/utils/ociutils"

	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/algorithmchoice"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras-server/pkg/constants"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
)

type DeltaManifestOptions struct {
	From string
	To   string
	algorithmchoice.AlgorithmChoice
}

type RegistryImpl struct {
	// TODO: replace with map of configured registries
	*remote.Registry
	registryUrl string
}

func NewRegistryDelegate(registryUrl string, registry *remote.Registry) RegistryDelegate {
	return &RegistryImpl{registryUrl: registryUrl, Registry: registry}
}

func (r *RegistryImpl) Resolve(image string, expectDigest bool) (oras.ReadOnlyTarget, string, v1.Descriptor, error) {
	repoName, tag, isDigest, err := apicommon.ParseOciImageString(image)
	if err != nil {
		return nil, "", v1.Descriptor{}, err
	}
	if expectDigest && !isDigest {
		return nil, "", v1.Descriptor{}, errors.New("expected digest")
	}
	repoNameTrimmed := strings.TrimPrefix(repoName, r.registryUrl+"/")
	//if repoNameTrimmed == repoName {
	//	return nil, "", v1.Descriptor{}, errors.New("invalid")
	//}
	ctx := context.Background()
	repository, err := r.Registry.Repository(ctx, repoNameTrimmed)
	if err != nil {
		return nil, "", v1.Descriptor{}, err
	}
	d, err := repository.Resolve(ctx, tag)
	if err != nil {
		return nil, "", v1.Descriptor{}, err
	}
	imageDigest := fmt.Sprintf("%s@sha256:%s", repoName, d.Digest.Encoded())
	return repository, imageDigest, d, nil
}

func (r *RegistryImpl) LoadManifest(target v1.Descriptor, source oras.ReadOnlyTarget) (v1.Manifest, error) {
	mfReader, err := source.Fetch(context.Background(), target)
	if err != nil {
		return v1.Manifest{}, err
	}
	defer funcutils.PanicOrLogOnErr(mfReader.Close, false, "failed to close reader")
	return ociutils.ParseManifest(mfReader)
}

func (r *RegistryImpl) LoadArtifact(mf v1.Manifest, source oras.ReadOnlyTarget) (io.ReadCloser, error) {
	if len(mf.Layers) != 1 {
		return nil, errors.New("expected single layer")
	}
	rc, err := source.Fetch(context.Background(), mf.Layers[0])
	if err != nil {
		return nil, err
	}
	return rc, nil
}

func (r *RegistryImpl) PushDelta(image string, manifOpts DeltaManifestOptions, content io.ReadCloser) error {
	repoName, tag, _, err := apicommon.ParseOciImageString(image)
	if err != nil {
		return err
	}

	repoNameTrimmed := strings.TrimPrefix(repoName, r.registryUrl+"/")
	//if repoNameTrimmed == repoName {
	//	return errors.New("invalid registry")
	//}
	ctx := context.Background()
	repository, err := r.Registry.Repository(ctx, repoNameTrimmed)
	if err != nil {
		return err
	}
	tempDir := os.TempDir()
	fp, err := os.CreateTemp(tempDir, "delta_*")
	if err != nil {
		return err
	}
	defer func() {
		if err := errors.Join(fp.Close(), os.Remove(fp.Name())); err != nil {
			logrus.WithError(err).Error("failed temp file clean up")
		}
	}()

	// we need to write it to the disk because we cannot push without knowing the hash of the data
	// hash file while writing it to the disk
	hasher := sha256.New()
	teeReader := io.NopCloser(io.TeeReader(content, hasher))
	n, err := io.Copy(fp, teeReader)
	if err != nil {
		return err
	}
	deltaDescriptor := v1.Descriptor{
		MediaType: manifOpts.GetMediaType(),
		Digest:    digest.NewDigest("sha256", hasher),
		Size:      n,
		URLs:      nil,
		Annotations: map[string]string{
			"org.opencontainers.image.title": "delta" + manifOpts.GetFileExt(),
		},
	}
	_, err = fp.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	err = repository.Push(ctx, deltaDescriptor, fp)
	if err != nil {
		return err
	}
	opts := oras.PackManifestOptions{
		Layers: []v1.Descriptor{deltaDescriptor},
		ManifestAnnotations: map[string]string{
			constants.DorasAnnotationFrom: manifOpts.From,
			constants.DorasAnnotationTo:   manifOpts.To,
		},
	}
	mfDescriptor, err := oras.PackManifest(ctx, repository, oras.PackManifestVersion1_1, "application/vnd.example+type", opts)
	if err != nil {
		return err
	}
	err = repository.Tag(ctx, mfDescriptor, tag)
	if err != nil {
		return err
	}
	logrus.Infof("created delta at %s with (tag/digest) (%s/%s)", image, tag, mfDescriptor.Digest.Encoded())
	return nil
}

func (r *RegistryImpl) PushDummy(image string, manifOpts DeltaManifestOptions) error {
	repoName, tag, _, err := apicommon.ParseOciImageString(image)
	if err != nil {
		return err
	}
	repoNameTrimmed := strings.TrimPrefix(repoName, r.registryUrl+"/")
	//if repoNameTrimmed == repoName {
	//	return errors.New("invalid registry")
	//}
	ctx := context.Background()
	repository, err := r.Registry.Repository(ctx, repoNameTrimmed)
	if err != nil {
		return err
	}
	// Dummy manifests use the empty descriptor and set a value in the annotations to indicate a dummy.
	opts := oras.PackManifestOptions{
		Layers: []v1.Descriptor{v1.DescriptorEmptyJSON},
		ManifestAnnotations: map[string]string{
			constants.DorasAnnotationFrom:    manifOpts.From,
			constants.DorasAnnotationTo:      manifOpts.To,
			constants.DorasAnnotationIsDummy: "true",
		},
	}
	mfDescriptor, err := oras.PackManifest(ctx, repository, oras.PackManifestVersion1_1, "application/vnd.example+type", opts)
	if err != nil {
		return err
	}
	err = repository.Tag(ctx, mfDescriptor, tag)
	if err != nil {
		return err
	}
	logrus.Infof("created dummy at %s", image)
	return nil
}

type RegistryDelegate interface {
	Resolve(image string, expectDigest bool) (oras.ReadOnlyTarget, string, v1.Descriptor, error)
	LoadManifest(target v1.Descriptor, source oras.ReadOnlyTarget) (v1.Manifest, error)
	LoadArtifact(mf v1.Manifest, source oras.ReadOnlyTarget) (io.ReadCloser, error)
	PushDelta(image string, manifOpts DeltaManifestOptions, content io.ReadCloser) error
	PushDummy(image string, manifOpts DeltaManifestOptions) error
}
