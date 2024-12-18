package deltaapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/internal/pkg/compression/gzip"
	delta2 "github.com/unbasical/doras-server/internal/pkg/delta"
	"github.com/unbasical/doras-server/internal/pkg/delta/bsdiff"
	"github.com/unbasical/doras-server/internal/pkg/delta/tardiff"
	"github.com/unbasical/doras-server/internal/pkg/utils/compressionutils"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras-server/pkg/constants"
	"github.com/unbasical/doras-server/pkg/delta"
	"io"
	"net/url"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/unbasical/doras-server/internal/pkg/api/registryexecuter"

	dorasErrors "github.com/unbasical/doras-server/internal/pkg/error"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
)

func BuildEdgeAPI(r *gin.Engine, config *apicommon.Config) *gin.Engine {
	log.Debug("Building edgeapi API")
	shared := registryexecuter.NewDeltaEngine(
		config.ArtifactStorage,
		config.RepoClients,
	)
	var reg *RegistryImpl
	var deltaEngine *deltaDelegateImpl
	for repoUrl, repoClient := range config.RepoClients {
		regTarget, err := remote.NewRegistry(repoUrl)
		if err != nil {
			panic(err)
		}
		regTarget.PlainHTTP = true
		regTarget.Client = repoClient
		reg = &RegistryImpl{
			Registry:    regTarget,
			registryUrl: repoUrl,
		}
		deltaEngine = &deltaDelegateImpl{
			baseUrl: repoUrl,
		}
	}

	r.Use(apicommon.SharedStateMiddleware(shared))
	log.Infof("%s", shared)
	edgeApiPath, err := url.JoinPath("/", apicommon.ApiBasePath, apicommon.DeltaApiPath)
	if err != nil {
		log.Fatal(err)
	}
	edgeAPI := r.Group(edgeApiPath)
	edgeAPI.GET("/", func(c *gin.Context) {
		dorasContext := GinDorasContext{c: c}
		readDeltaNew(reg, deltaEngine, &dorasContext)
	})
	return r
}

type RegistryImpl struct {
	// TODO: replace with map of configured registries
	*remote.Registry
	registryUrl string
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
	return parseManifest(mfReader)
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

func (r *RegistryImpl) PushDelta(image string, manifOpts registryexecuter.DeltaManifestOptions, content io.ReadCloser) error {
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
			log.WithError(err).Error("failed temp file clean up")
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
	log.Infof("created delta at %s with (tag/digest) (%s/%s)", image, tag, mfDescriptor.Digest.Encoded())
	return nil
}

func (r *RegistryImpl) PushDummy(image string, manifOpts registryexecuter.DeltaManifestOptions) error {
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
	log.Infof("created dummy at %s", image)
	return nil
}

type RegistryDelegate interface {
	Resolve(image string, expectDigest bool) (oras.ReadOnlyTarget, string, v1.Descriptor, error)
	LoadManifest(target v1.Descriptor, source oras.ReadOnlyTarget) (v1.Manifest, error)
	LoadArtifact(mf v1.Manifest, source oras.ReadOnlyTarget) (io.ReadCloser, error)
	PushDelta(image string, manifOpts registryexecuter.DeltaManifestOptions, content io.ReadCloser) error
	PushDummy(image string, manifOpts registryexecuter.DeltaManifestOptions) error
}

type deltaDelegateImpl struct {
	baseUrl string
}

func (d *deltaDelegateImpl) IsDummy(mf v1.Manifest) (isDummy bool, expired bool) {
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

func (d *deltaDelegateImpl) GetDeltaLocation(deltaMf registryexecuter.DeltaManifestOptions) (string, error) {
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

func (d *deltaDelegateImpl) CreateDelta(from, to io.ReadCloser, manifOpts registryexecuter.DeltaManifestOptions) (io.ReadCloser, error) {
	deltaReader, err := manifOpts.Differ.Diff(from, to)
	if err != nil {
		return nil, err
	}
	compressedDelta, err := manifOpts.Compressor.Compress(deltaReader)
	if err != nil {
		return nil, err
	}
	return compressedDelta, nil
}

type DeltaDelegate interface {
	IsDummy(mf v1.Manifest) (isDummy bool, expired bool)
	GetDeltaLocation(deltaMf registryexecuter.DeltaManifestOptions) (string, error)
	CreateDelta(from, to io.ReadCloser, manifOpts registryexecuter.DeltaManifestOptions) (io.ReadCloser, error)
}

func readDeltaNew(registry RegistryDelegate, delegate DeltaDelegate, apiDelegate APIDelegate) {
	fromDigest, toTarget, acceptedAlgorithms, err := apiDelegate.ExtractParams()
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, "")
		return
	}
	srcFrom, fromImage, fromDescriptor, err := registry.Resolve(fromDigest, true)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInvalidOciImage, fromDigest)
		return
	}
	srcTo, toImage, toDescriptor, err := registry.Resolve(toTarget, false)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInvalidOciImage, toTarget)
		return
	}
	mfFrom, err := registry.LoadManifest(fromDescriptor, srcFrom)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, "")
		return
	}
	mfTo, err := registry.LoadManifest(toDescriptor, srcTo)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, "")
		return
	}
	if err := checkCompatability(&mfFrom, &mfTo); err != nil {
		apiDelegate.HandleError(dorasErrors.ErrIncompatibleArtifacts, err.Error())
		return
	}
	manifOpts := registryexecuter.DeltaManifestOptions{
		From:            fromImage,
		To:              toImage,
		AlgorithmChoice: chooseAlgorithm(acceptedAlgorithms, &mfFrom, &mfTo),
	}
	deltaImage, err := delegate.GetDeltaLocation(manifOpts)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, "")
		return
	}
	// create dummy manifest
	deltaImageWithTag := deltaImage + ":" + manifOpts.GetTag()
	log.Debugf("looking for delta at %s", deltaImageWithTag)
	if deltaSrc, deltaImageDigest, deltaDescriptor, err := registry.Resolve(deltaImageWithTag, false); err == nil {
		log.Debugf("found delta at %s", deltaImageDigest)
		mfDelta, err := registry.LoadManifest(deltaDescriptor, deltaSrc)
		if err != nil {
			apiDelegate.HandleError(dorasErrors.ErrInternal, "")
			return
		}
		dummy, expired := delegate.IsDummy(mfDelta)
		// the delta has been created
		if !dummy {
			// All deltas that get actually served get served here.
			apiDelegate.HandleSuccess(apicommon.ReadDeltaResponse{
				TargetImage: toImage,
				DeltaImage:  deltaImageDigest,
			})
			return
		}
		// dummy exists and has not expired -> someone else is working on creating this delta
		if !expired {
			apiDelegate.HandleAccepted()
			return
		}
	} else {
		log.Debugf("failed to resolve delta %v", err)
	}

	err = registry.PushDummy(deltaImageWithTag, manifOpts)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, "")
		return
	}
	rcFrom, err := registry.LoadArtifact(mfFrom, srcFrom)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, "")
		return
	}
	rcTo, err := registry.LoadArtifact(mfTo, srcTo)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, "")
		return
	}
	rcDelta, err := delegate.CreateDelta(rcFrom, rcTo, manifOpts)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, "")
		return
	}
	// asynchronously create delta
	go func() {
		defer funcutils.PanicOrLogOnErr(rcTo.Close, false, "failed to close reader")
		defer funcutils.PanicOrLogOnErr(rcFrom.Close, false, "failed to close reader")
		log.Debug(deltaImage)
		err := registry.PushDelta(deltaImageWithTag, manifOpts, rcDelta)
		if err != nil {
			log.WithError(err).Error("failed to push delta")
			return
		}
	}()
	// tell client has the delta has been accepted
	apiDelegate.HandleAccepted()
}

func chooseAlgorithm(acceptedAlgorithms []string, mfFrom, mfTo *v1.Manifest) registryexecuter.AlgorithmChoice {
	_ = mfTo

	algorithm := registryexecuter.AlgorithmChoice{
		Differ:     bsdiff.NewCreator(),
		Compressor: compressionutils.NewNopCompressor(),
	}
	if mfFrom.Layers[0].Annotations[delta2.ContentUnpack] == "true" && slices.Contains(acceptedAlgorithms, "tardiff") {
		algorithm.Differ = tardiff.NewCreator()
	}
	if slices.Contains(acceptedAlgorithms, "gzip") {
		algorithm.Compressor = gzip.NewCompressor()
	}
	return algorithm
}

func parseManifest(content io.Reader) (v1.Manifest, error) {
	var mf v1.Manifest
	err := json.NewDecoder(content).Decode(&mf)
	if err != nil {
		return v1.Manifest{}, err
	}
	return mf, nil
}

type DeltaPolicy interface {
	ChooseDiffer(from v1.Descriptor, descriptor v1.Descriptor, acceptedAlgorithms []string) (delta.Differ, error)
}

func checkCompatability(from *v1.Manifest, to *v1.Manifest) error {
	if len(from.Layers) != len(to.Layers) {
		return errors.New("incompatible amount of layers")
	}
	if from.Annotations[delta2.ContentUnpack] != to.Annotations[delta2.ContentUnpack] {
		return errors.New("incompatible artifacts")
	}
	return nil
}
