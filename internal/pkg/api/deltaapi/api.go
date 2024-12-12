package deltaapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	delta2 "github.com/unbasical/doras-server/internal/pkg/delta"
	"github.com/unbasical/doras-server/internal/pkg/utils/funcutils"
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
	_, rcFrom, err := shared.ResolveAndLoad(fromDigest, true)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInvalidOciImage, fromDigest)
		return
	}
	defer funcutils.PanicOrLogOnErr(rcFrom.Close, false, "failed to close reader")

	_, rcTo, err := shared.ResolveAndLoad(toTarget, false)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, toTarget)
		return
	}
	defer funcutils.PanicOrLogOnErr(rcFrom.Close, false, "failed to close reader")

	mfFrom, err := ParseManifest(rcFrom)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, "invalid manifest")
		return
	}
	mfTo, err := ParseManifest(rcTo)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, "invalid manifest")
		return
	}
	if err := checkCompatability(&mfFrom, &mfTo); err != nil {
		apiDelegate.HandleError(dorasErrors.ErrIncompatibleArtifacts, err.Error())
		return
	}
	// TODO extract parameter verification from ReadDeltaImpl
	deltaResponse, err, msg := shared.ReadDeltaImpl(fromDigest, toTarget)
	if err != nil {
		apiDelegate.HandleError(err, msg)
		return
	}
	apiDelegate.HandleSuccess(*deltaResponse)
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
