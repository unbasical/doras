package api

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func BuildEdgeAPI(r *gin.Engine) *gin.Engine {
	log.Debug("Building edge API")
	edgeAPI := r.Group("/edge/artifacts/")
	edgeAPI.POST("delta/create", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	edgeAPI.GET("delta", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	edgeAPI.GET("full", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	return r
}
