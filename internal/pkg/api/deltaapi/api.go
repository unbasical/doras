package deltaapi

import (
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/unbasical/doras-server/pkg/delta"
	"net/url"

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
	_, err = shared.Resolve(fromDigest, true)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInvalidOciImage, fromDigest)
		return
	}
	_, err = shared.Resolve(toTarget, false)
	if err != nil {
		apiDelegate.HandleError(dorasErrors.ErrInternal, toTarget)
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

type DeltaPolicy interface {
	ChooseDiffer(from v1.Descriptor, descriptor v1.Descriptor, acceptedAlgorithms []string) (delta.Differ, error)
}
