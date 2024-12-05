package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
	"github.com/unbasical/doras-server/internal/pkg/api/cloudapi"
	"github.com/unbasical/doras-server/internal/pkg/api/edgeapi"
)

func BuildApp(config *apicommon.Config) *gin.Engine {
	log.Debug("Building app")
	r := gin.Default()
	r = edgeapi.BuildEdgeAPI(r, config)
	r = cloudapi.BuildCloudAPI(r, config)
	r.GET("/api/v1/ping", ping)

	return r
}

// @BasePath /api/v1/ping
// PingExample godoc
// @Summary ping example
// @Schemes
// @Description do ping
// @Tags example
// @Accept json
// @Produce json
// @Success 200 {string} {"message":"pong"}
// @Router /api/v1/ping [get]
func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}
