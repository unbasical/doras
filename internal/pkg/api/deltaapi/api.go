package deltaapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	delta2 "github.com/unbasical/doras-server/internal/pkg/delta"
	"github.com/unbasical/doras-server/pkg/delta"

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
	r.Use(apicommon.SharedStateMiddleware(shared))
	log.Infof("%s", shared)
	edgeApiPath, err := url.JoinPath("/", apicommon.ApiBasePath, apicommon.DeltaApiPath)
	if err != nil {
		log.Fatal(err)
	}
	edgeAPI := r.Group(edgeApiPath)
	edgeAPI.GET("/", func(c *gin.Context) {
		dorasContext := GinDorasContext{c: c}
		readDelta(&dorasContext)
	})
	return r
}

// readDelta
// Stores the artifact provided as a file in the request body.
func readDelta(apiDelegate APIDelegate) {
	shared, err := apiDelegate.ExtractState()
	if err != nil {
		apiDelegate.HandleError(err, err.Error())
		return
	}
	fromDigest, toTarget, _, err := apiDelegate.ExtractParams()
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, "")
		return
	}
	dFrom, mfFrom, source, err := shared.ResolveAndLoadManifest(fromDigest, true)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInvalidOciImage, fromDigest)
		return
	}

	dTo, mfTo, _, err := shared.ResolveAndLoadManifest(toTarget, false)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, toTarget)
		return
	}

	if err := checkCompatability(&mfFrom, &mfTo); err != nil {
		apiDelegate.HandleError(dorasErrors.ErrIncompatibleArtifacts, err.Error())
		return
	}
	// TODO extract parameter verification from ReadDeltaImpl
	deltaDescriptor, err, msg := shared.ReadDeltaImpl(source, dFrom, dTo)
	if err != nil {
		apiDelegate.HandleError(err, msg)
		return
	}
	name, _, _, _ := apicommon.ParseOciImageString(toTarget)
	toTarget = fmt.Sprintf("%s@sha256:%s", name, dTo.Digest.Encoded())
	deltaResponse := apicommon.ReadDeltaResponse{
		TargetImage:     toTarget,
		DeltaDescriptor: *deltaDescriptor,
	}

	apiDelegate.HandleSuccess(deltaResponse)
}

func ParseManifest(content io.Reader) (v1.Manifest, error) {
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
