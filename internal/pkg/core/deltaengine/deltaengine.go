package deltaengine

import (
	"errors"
	"fmt"

	"oras.land/oras-go/v2/registry/remote/auth"

	apidelegate "github.com/unbasical/doras-server/internal/pkg/delegates/api"
	deltadelegate "github.com/unbasical/doras-server/internal/pkg/delegates/delta"
	registrydelegate "github.com/unbasical/doras-server/internal/pkg/delegates/registry"
	"github.com/unbasical/doras-server/internal/pkg/utils/ociutils"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/algorithmchoice"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
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

// checkRepoCompatability ensures that the two provided images are from the same repository.
func checkRepoCompatability(a, b string) error {
	nameA, _, _, err := ociutils.ParseOciImageString(a)
	if err != nil {
		return err
	}
	nameB, _, _, err := ociutils.ParseOciImageString(b)
	if err != nil {
		return err
	}
	if nameA != nameB {
		return fmt.Errorf("requested images are not from the same repository")
	}
	return nil
}

func readDelta(registry registrydelegate.RegistryDelegate, delegate deltadelegate.DeltaDelegate, apiDelegate apidelegate.APIDelegate) {
	fromDigest, toTarget, acceptedAlgorithms, err := apiDelegate.ExtractParams()
	if err != nil {
		log.WithError(err).Error("Error extracting parameters")
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}
	err = checkRepoCompatability(fromDigest, toTarget)
	if err != nil {
		log.WithError(err).Error("Error checking repo compatibility")
		apiDelegate.HandleError(error2.ErrInternal, err.Error())
		return
	}

	var creds auth.CredentialFunc
	if clientAuth, err := apiDelegate.ExtractClientAuth(); err != nil {
		log.WithError(err).Debug("Error extracting client token")
	} else {
		repoUrl, err := ociutils.ParseOciUrl(fromDigest)
		if err != nil {
			log.WithError(err).Error("Error extracting repo url")
			apiDelegate.HandleError(error2.ErrInternal, "")
			return
		}
		creds, err = clientAuth.CredentialFunc(repoUrl.Host)
		if err != nil {
			log.WithError(err).Error("Error extracting credentials")
			apiDelegate.HandleError(error2.ErrInternal, "")
			return
		}
	}

	// resolve images to ensure they exist
	srcFrom, fromImage, fromDescriptor, err := registry.Resolve(fromDigest, true, creds)
	if err != nil {
		log.WithError(err).Errorf("Error resolving target %q", fromDigest)
		apiDelegate.HandleError(error2.ErrInvalidOciImage, fromDigest)
		return
	}
	srcTo, toImage, toDescriptor, err := registry.Resolve(toTarget, false, creds)
	if err != nil {
		log.WithError(err).Errorf("Error resolving target %q", toTarget)
		apiDelegate.HandleError(error2.ErrInvalidOciImage, toTarget)
		return
	}

	// load manifests to check for compatability and algorithm selection
	mfFrom, err := registry.LoadManifest(fromDescriptor, srcFrom)
	if err != nil {
		log.WithError(err).Error("Error loading manifest")
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}
	mfTo, err := registry.LoadManifest(toDescriptor, srcTo)
	if err != nil {
		log.WithError(err).Error("Error loading manifest")
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}
	if err := checkCompatability(&mfFrom, &mfTo); err != nil {
		apiDelegate.HandleError(error2.ErrIncompatibleArtifacts, err.Error())
		return
	}
	manifOpts := registrydelegate.DeltaManifestOptions{
		From:         fromImage,
		To:           toImage,
		DifferChoice: algorithmchoice.ChooseAlgorithm(acceptedAlgorithms, &mfFrom, &mfTo),
	}

	deltaImage, err := delegate.GetDeltaLocation(manifOpts)
	if err != nil {
		log.WithError(err).Error("failed to get delta location")
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}

	// Ensure we do not push the delta to a different registry than the original source images.
	// We only have to check on of the images because we already previously ensured the sources images are in the same repo.
	// This should be refactored in the future because it is a bit hacky.
	err = ociutils.CheckRegistryMatch(fromDigest, deltaImage)
	if err != nil {
		log.WithError(err).Error("source images not in the same registry as the delta image")
		apiDelegate.HandleError(error2.ErrBadRequest, "source images not in the same registry as the delta image")
		return
	}

	// create dummy manifest
	deltaImageWithTag := deltaImage
	log.Debugf("looking for delta at %s", deltaImageWithTag)
	if deltaSrc, deltaImageDigest, deltaDescriptor, err := registry.Resolve(deltaImageWithTag, false, creds); err == nil {
		log.Debugf("found delta at %s", deltaImageDigest)
		mfDelta, err := registry.LoadManifest(deltaDescriptor, deltaSrc)
		if err != nil {
			log.WithError(err).Error("failed to load manifest")
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
		log.Debugf("failed to resolve delta %v", err)
	}

	// Push dummy to communicate that someone is working on the delta.
	err = registry.PushDummy(deltaImageWithTag, manifOpts)
	if err != nil {
		log.WithError(err).Error("failed to push dummy")
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}

	// load artifacts for delta calculation
	rcFrom, err := registry.LoadArtifact(mfFrom, srcFrom)
	if err != nil {
		log.WithError(err).Error("failed to load 'from' artifact")
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}
	rcTo, err := registry.LoadArtifact(mfTo, srcTo)
	if err != nil {
		log.WithError(err).Error("failed to load 'to' artifact")
		apiDelegate.HandleError(error2.ErrInternal, "")
		return
	}

	// asynchronously create delta
	go func() {
		defer funcutils.PanicOrLogOnErr(rcTo.Close, false, "failed to close reader")
		defer funcutils.PanicOrLogOnErr(rcFrom.Close, false, "failed to close reader")
		err := delegate.CreateDelta(rcFrom, rcTo, manifOpts, registry)
		if err != nil {
			log.WithError(err).Error("failed to create delta")
			apiDelegate.HandleError(error2.ErrInternal, "")
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
