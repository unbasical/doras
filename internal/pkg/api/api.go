package api

import (
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/aliasing"
	"github.com/unbasical/doras-server/internal/pkg/storage"
	"net/http"
)

type Config struct {
	ArtifactStorage storage.ArtifactStorage
	Aliaser         aliasing.Aliasing
}

func BuildApp(config *Config) *gin.Engine {
	log.Debug("Building app")
	r := gin.Default()
	r = BuildEdgeAPI(r, config)
	r = BuildCloudAPI(r, config)
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	return r
}
