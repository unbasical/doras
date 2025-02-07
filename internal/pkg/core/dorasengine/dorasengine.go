package dorasengine

import (
	"context"
	"errors"
	"fmt"
	"oras.land/oras-go/v2/registry/remote/auth"
	"strings"
	"sync"

	apidelegate "github.com/unbasical/doras/internal/pkg/delegates/api"
	deltadelegate "github.com/unbasical/doras/internal/pkg/delegates/delta"
	registrydelegate "github.com/unbasical/doras/internal/pkg/delegates/registry"
	"github.com/unbasical/doras/internal/pkg/utils/ociutils"

	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/algorithmchoice"
	"github.com/unbasical/doras/internal/pkg/api/apicommon"
	error2 "github.com/unbasical/doras/internal/pkg/error"
	"github.com/unbasical/doras/internal/pkg/utils/funcutils"
	"github.com/unbasical/doras/pkg/constants"
)

// Engine handles delta requests, including graceful shutdowns.
// Errors and responses are handled by apidelegate.APIDelegate implementations.
type Engine interface {
	HandleReadDelta(apiDeletgate apidelegate.APIDelegate)
	Stop(ctx context.Context)
}

type engine struct {
	registry          registrydelegate.RegistryDelegate
	delegate          deltadelegate.DeltaDelegate
	requireClientAuth bool
	wg                *sync.WaitGroup
}

// NewEngine construct a new dorasengine.Engine with the given delegates.
func NewEngine(registry registrydelegate.RegistryDelegate, delegate deltadelegate.DeltaDelegate, requireClientAuth bool) Engine {
	return &engine{
		registry:          registry,
		delegate:          delegate,
		wg:                &sync.WaitGroup{},
		requireClientAuth: requireClientAuth,
	}
}
func (d *engine) Stop(ctx context.Context) {
	doneChan := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(doneChan)
	}()
	select {
	case <-ctx.Done():
		log.Debug(ctx.Err())
	case <-doneChan:
		log.Debug("all delta requests have been served")
	}
}

func (d *engine) HandleReadDelta(apiDeletgate apidelegate.APIDelegate) {
	ctx := context.WithValue(context.Background(), "wg", d.wg)
	readDelta(ctx, d.registry, d.delegate, apiDeletgate, d.requireClientAuth)
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

//nolint:revive // This rule is disabled to get around complexity linter errors. Reducing the complexity of this function is difficult. Refer to the Doras specs in the file docs/delta-creation-spec.md for more information on the semantics of this god function.
func readDelta(ctx context.Context, registry registrydelegate.RegistryDelegate, delegate deltadelegate.DeltaDelegate, apiDelegate apidelegate.APIDelegate, requireClientAuth bool) {
	wg, ok := ctx.Value("wg").(*sync.WaitGroup)
	if !ok {
		panic("missing wait group in context")
	}
	wg.Add(1)
	defer wg.Done()
	fromDigest, toTarget, acceptedAlgorithms, err := apiDelegate.ExtractParams()
	if err != nil {
		log.WithError(err).Error("Error extracting parameters")
		if requireClientAuth {
			apiDelegate.HandleError(error2.ErrUnauthorized, "")
			return
		}
		apiDelegate.HandleError(error2.ErrBadRequest, "")
		return
	}
	var creds auth.CredentialFunc
	clientAuth, err := apiDelegate.ExtractClientAuth()
	if err != nil {
		log.WithError(err).Debug("Error extracting client token")
		if requireClientAuth {
			apiDelegate.HandleError(error2.ErrUnauthorized, "")
			return
		}
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

	err = checkRepoCompatability(fromDigest, toTarget)
	if err != nil {
		log.WithError(err).Error("Error checking repo compatibility")
		apiDelegate.HandleError(error2.ErrBadRequest, "images are not in the same repository")
		return
	}

	// resolve images to ensure they exist
	srcFrom, fromImage, fromDescriptor, err := registry.Resolve(fromDigest, true, creds)
	if err != nil {
		log.WithError(err).Errorf("Error resolving target %q", fromDigest)
		// assume there is the string "unauthorized" in the error message when there is an auth failure
		if strings.Contains(err.Error(), "unauthorized") {
			apiDelegate.HandleError(error2.ErrUnauthorized, "")
			return
		}
		if errors.Is(err, error2.ErrExpectedDigest) {
			apiDelegate.HandleError(error2.ErrBadRequest, "expected digest")
			return
		}
		apiDelegate.HandleError(error2.ErrFailedToResolve, fromDigest)
		return
	}
	srcTo, toImage, toDescriptor, err := registry.Resolve(toTarget, false, creds)
	if err != nil {
		log.WithError(err).Errorf("Error resolving target %q", toTarget)
		// assume there is the string "unauthorized" in the error message when there is an auth failure
		if strings.Contains(err.Error(), "unauthorized") {
			apiDelegate.HandleError(error2.ErrUnauthorized, "")
			return
		}
		apiDelegate.HandleError(error2.ErrInvalidOciImage, toTarget)
		return
	}
	if toDescriptor.Digest == fromDescriptor.Digest {
		log.Debugf("got request for images with identical digests %v, %v", fromDescriptor.Digest, toDescriptor.Digest)
		apiDelegate.HandleError(error2.ErrIncompatibleArtifacts, "from and to image are identical")
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
		log.WithError(err).Debug("received request for incompatible artifacts")
		apiDelegate.HandleError(error2.ErrIncompatibleArtifacts, "cannot build a delta from images")
		return
	}
	manifOpts := registrydelegate.DeltaManifestOptions{
		From:         fromImage,
		To:           toImage,
		DifferChoice: algorithmchoice.ChooseAlgorithms(acceptedAlgorithms, &mfFrom, &mfTo),
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
		wg.Add(1)
		defer wg.Done()
		defer funcutils.PanicOrLogOnErr(rcTo.Close, false, "failed to close reader")
		defer funcutils.PanicOrLogOnErr(rcFrom.Close, false, "failed to close reader")
		err := delegate.CreateDelta(ctx, rcFrom, rcTo, manifOpts, registry)
		if err != nil {
			log.WithError(err).Error("failed to create delta")
			return
		}
	}()
	// tell client has the delta has been accepted
	apiDelegate.HandleAccepted()
}

func checkCompatability(from *ociutils.Manifest, to *ociutils.Manifest) error {
	if len(from.Layers) != len(to.Layers) {
		return errors.New("incompatible amount of layers")
	}
	if from.Annotations[constants.OrasContentUnpack] != to.Annotations[constants.OrasContentUnpack] {
		return errors.New("incompatible artifacts")
	}
	return nil
}
