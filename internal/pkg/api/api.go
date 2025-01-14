package api

import (
	"net/http"
	"net/url"
	"time"

	"github.com/unbasical/doras-server/internal/pkg/api/gindelegate"

	deltadelegate "github.com/unbasical/doras-server/internal/pkg/delegates/delta"
	registrydelegate "github.com/unbasical/doras-server/internal/pkg/delegates/registry"

	"github.com/unbasical/doras-server/internal/pkg/core/dorasengine"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras-server/internal/pkg/api/apicommon"
)

func logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		c.Next()
		latency := time.Since(startTime)
		log.WithFields(log.Fields{
			"status":    c.Writer.Status(),
			"method":    c.Request.Method,
			"path":      c.Request.URL.Path,
			"ip":        c.ClientIP(),
			"latency":   latency,
			"error":     c.Errors.ByType(gin.ErrorTypePrivate).String(),
			"userAgent": c.Request.UserAgent(),
		}).Info("Request completed")
	}
}

// BuildApp return an engine that when ran servers the Doras API.
// Uses the provided configuration to set up logging, storage and other things.
func BuildApp(config *apicommon.Config) *gin.Engine {
	log.Debug("Building app")
	gin.DisableConsoleColor()
	r := gin.New()
	r.Use(
		logger(),
	)
	r = buildEdgeAPI(r, config)
	r.GET("/api/v1/ping", ping)

	return r
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}

// buildEdgeAPI sets up the API which handles delta requests.
func buildEdgeAPI(r *gin.Engine, config *apicommon.Config) *gin.Engine {
	log.Debug("Building edge API")

	var reg registrydelegate.RegistryDelegate
	var deltaDelegate deltadelegate.DeltaDelegate
	for repoUrl, repoClient := range config.RepoClients {
		regTarget, err := remote.NewRegistry(repoUrl)
		if err != nil {
			panic(err)
		}
		regTarget.PlainHTTP = true
		regTarget.Client = repoClient
		reg = registrydelegate.NewRegistryDelegate(repoUrl, regTarget)
		deltaDelegate = deltadelegate.NewDeltaDelegate(repoUrl)
	}
	dorasEngine := dorasengine.NewEngine(reg, deltaDelegate)

	edgeApiPath, err := url.JoinPath("/", apicommon.ApiBasePathV1, apicommon.DeltaApiPath)
	if err != nil {
		log.Error(err)
		panic(err)
	}
	edgeAPI := r.Group(edgeApiPath)
	edgeAPI.GET("/", func(c *gin.Context) {
		apiDelegate := gindelegate.NewDelegate(c)
		dorasEngine.HandleReadDelta(apiDelegate)
	})
	return r
}
