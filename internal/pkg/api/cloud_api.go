package api

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func BuildCloudAPI(r *gin.Engine) *gin.Engine {
	log.Debug("Building cloud API")
	artifactsAPI := r.Group("/api/artifacts")

	// TODO: parse path
	artifactsAPI.PUT("", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	// List all artifacts
	artifactsAPI.GET("", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	// Path to specific artifact
	artifactsAPI.GET("", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	artifactsAPI.PUT("", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	artifactsAPI.PATCH("", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	artifactsAPI.DELETE("", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, "not implemented")
	})
	return r
}
