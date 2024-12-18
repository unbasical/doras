package deltaengine

import (
	"errors"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/algorithmchoice"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/delegates/api"
	"github.com/unbasical/doras-server/internal/pkg/delegates/delta"
	"github.com/unbasical/doras-server/internal/pkg/delegates/registry"
	error2 "github.com/unbasical/doras-server/internal/pkg/error"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras-server/pkg/constants"
)

type DeltaEngine interface {
	HandleReadDelta(apiDeletgate apidelegate.APIDelegate)
}

type deltaEngine struct {
	registry registrydelegate.RegistryDelegate
	delegate deltadelegate.DeltaDelegate
}

func (d *deltaEngine) HandleReadDelta(apiDeletgate apidelegate.APIDelegate) {
	readDelta(d.registry, d.delegate, apiDeletgate)
}

func NewDeltaEngine(registry registrydelegate.RegistryDelegate, delegate deltadelegate.DeltaDelegate) DeltaEngine {
	return &deltaEngine{registry: registry, delegate: delegate}
}

func readDelta(registry registrydelegate.RegistryDelegate, delegate deltadelegate.DeltaDelegate, apiDelegate apidelegate.APIDelegate) {
	fromDigest, toTarget, acceptedAlgorithms, err := apiDelegate.ExtractParams()
	if err != nil {
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}

	// resolve images to ensure they exist
	srcFrom, fromImage, fromDescriptor, err := registry.Resolve(fromDigest, true)
	if err != nil {
		apiDelegate.HandleError(error2.ErrInvalidOciImage, fromDigest)
		return
	}
	srcTo, toImage, toDescriptor, err := registry.Resolve(toTarget, false)
	if err != nil {
		apiDelegate.HandleError(error2.ErrInvalidOciImage, toTarget)
		return
	}

	// load manifests to check for compatability and algorithm selection
	mfFrom, err := registry.LoadManifest(fromDescriptor, srcFrom)
	if err != nil {
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}
	mfTo, err := registry.LoadManifest(toDescriptor, srcTo)
	if err != nil {
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}
	if err := checkCompatability(&mfFrom, &mfTo); err != nil {
		apiDelegate.HandleError(error2.ErrIncompatibleArtifacts, err.Error())
		return
	}
	manifOpts := registrydelegate.DeltaManifestOptions{
		From:            fromImage,
		To:              toImage,
		AlgorithmChoice: algorithmchoice.ChooseAlgorithm(acceptedAlgorithms, &mfFrom, &mfTo),
	}

	deltaImage, err := delegate.GetDeltaLocation(manifOpts)
	if err != nil {
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}
	// create dummy manifest
	deltaImageWithTag := deltaImage + ":" + manifOpts.GetTag()
	logrus.Debugf("looking for delta at %s", deltaImageWithTag)
	if deltaSrc, deltaImageDigest, deltaDescriptor, err := registry.Resolve(deltaImageWithTag, false); err == nil {
		logrus.Debugf("found delta at %s", deltaImageDigest)
		mfDelta, err := registry.LoadManifest(deltaDescriptor, deltaSrc)
		if err != nil {
			apiDelegate.HandleError(error2.ErrInternal, "")
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
		logrus.Debugf("failed to resolve delta %v", err)
	}

	// Push dummy to communicate that someone is working on the delta.
	err = registry.PushDummy(deltaImageWithTag, manifOpts)
	if err != nil {
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}

	// load artifacts for delta calculation
	rcFrom, err := registry.LoadArtifact(mfFrom, srcFrom)
	if err != nil {
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}
	rcTo, err := registry.LoadArtifact(mfTo, srcTo)
	if err != nil {
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}
	rcDelta, err := delegate.CreateDelta(rcFrom, rcTo, manifOpts)
	if err != nil {
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}
	// asynchronously create delta
	go func() {
		defer funcutils.PanicOrLogOnErr(rcTo.Close, false, "failed to close reader")
		defer funcutils.PanicOrLogOnErr(rcFrom.Close, false, "failed to close reader")
		logrus.Debug(deltaImage)
		err := registry.PushDelta(deltaImageWithTag, manifOpts, rcDelta)
		if err != nil {
			logrus.WithError(err).Error("failed to push delta")
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
	if from.Annotations[constants.ContentUnpack] != to.Annotations[constants.ContentUnpack] {
		return errors.New("incompatible artifacts")
	}
	return nil
}
