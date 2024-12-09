package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/api/deltaapi"
)

func BuildApp(config *apicommon.Config) *gin.Engine {
	log.Debug("Building app")
	r := gin.Default()
	r = deltaapi.BuildEdgeAPI(r, config)
	r.GET("/api/v1/ping", ping)

	return r
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}
