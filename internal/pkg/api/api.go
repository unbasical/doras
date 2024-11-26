package api

import (
	"net/http"

	"github.com/unbasical/doras-server/internal/pkg/docs"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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

	docs.SwaggerInfo.BasePath = "/api/v1"
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
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
