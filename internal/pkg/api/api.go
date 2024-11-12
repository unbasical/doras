package api

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/api/cloudapi"
	"github.com/unbasical/doras-server/internal/pkg/api/edgeapi"
	"net/http"
)

func BuildApp(config *apicommon.Config) *gin.Engine {
	log.Debug("Building app")
	r := gin.Default()
	r = edgeapi.BuildEdgeAPI(r, config)
	r = cloudapi.BuildCloudAPI(r, config)
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	return r
}
