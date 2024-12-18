package deltaapi

import (
	"errors"
	"net/url"

	"github.com/unbasical/doras-server/internal/pkg/algorithmchoice"
	"github.com/unbasical/doras-server/internal/pkg/api/deltadelegate"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	delta2 "github.com/unbasical/doras-server/internal/pkg/delta"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/unbasical/doras-server/internal/pkg/api/registrydelegate"

	dorasErrors "github.com/unbasical/doras-server/internal/pkg/error"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
)

func BuildEdgeAPI(r *gin.Engine, config *apicommon.Config) *gin.Engine {
	log.Debug("Building edgeapi API")

	var reg registrydelegate.RegistryDelegate
	var deltaEngine deltadelegate.DeltaDelegate
	for repoUrl, repoClient := range config.RepoClients {
		regTarget, err := remote.NewRegistry(repoUrl)
		if err != nil {
			panic(err)
		}
		regTarget.PlainHTTP = true
		regTarget.Client = repoClient
		reg = registrydelegate.NewRegistryDelegate(repoUrl, regTarget)
		deltaEngine = deltadelegate.NewDeltaDelegate(repoUrl)
	}

	edgeApiPath, err := url.JoinPath("/", apicommon.ApiBasePath, apicommon.DeltaApiPath)
	if err != nil {
		log.Fatal(err)
	}
	edgeAPI := r.Group(edgeApiPath)
	edgeAPI.GET("/", func(c *gin.Context) {
		dorasContext := GinDorasContext{c: c}
		readDelta(reg, deltaEngine, &dorasContext)
	})
	return r
}

func readDelta(registry registrydelegate.RegistryDelegate, delegate deltadelegate.DeltaDelegate, apiDelegate APIDelegate) {
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
	manifOpts := registrydelegate.DeltaManifestOptions{
		From:            fromImage,
		To:              toImage,
		AlgorithmChoice: algorithmchoice.ChooseAlgorithm(acceptedAlgorithms, &mfFrom, &mfTo),
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

func checkCompatability(from *v1.Manifest, to *v1.Manifest) error {
	if len(from.Layers) != len(to.Layers) {
		return errors.New("incompatible amount of layers")
	}
	if from.Annotations[delta2.ContentUnpack] != to.Annotations[delta2.ContentUnpack] {
		return errors.New("incompatible artifacts")
	}
	return nil
}
