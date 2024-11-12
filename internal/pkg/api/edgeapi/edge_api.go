package edgeapi

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
)

type EdgeAPI struct {
	artifactStorageProvider apicommon.DorasStorage
}

func BuildEdgeAPI(r *gin.Engine, config *apicommon.Config) *gin.Engine {
	log.Debug("Building edge API")
	shared := &EdgeAPI{
		artifactStorageProvider: config.ArtifactStorage,
	}
	log.Infof("%s", shared)
	edgeAPI := r.Group("/edge/artifacts")

	edgeAPI.POST("/delta", func(c *gin.Context) {
		panic("todo")
	})

	edgeAPI.GET("/delta/:identifier", func(c *gin.Context) {
		panic("todo")

		// readDelta(shared, c)
	})

	edgeAPI.GET("/full/:identifier", func(c *gin.Context) {
		panic("todo")
		// readFull(shared, c)
	})
	return r
}
