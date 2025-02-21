package api

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/unbasical/doras/internal/pkg/core/metrics"
	"net/http"
	"net/url"
	"time"

	"github.com/unbasical/doras/internal/pkg/api/gindelegate"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/api/apicommon"
	"github.com/unbasical/doras/internal/pkg/core/dorasengine"
)

func init() {
	metrics.DorasRegisterer.MustRegister(metrics.HttpRequestsTotal)
	metrics.DorasRegisterer.MustRegister(metrics.HttpRequestDuration)
}

// logger creates gin.HandlerFunc that uses logrus for logging.
func logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		c.Next()
		latency := time.Since(startTime)
		if c.Request.URL.Path == "/api/v1/ping" {
			return
		}
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
func BuildApp(engine dorasengine.Engine, exposeMetrics bool) *gin.Engine {
	log.Debug("Building app")
	gin.DisableConsoleColor()
	r := gin.New()
	r.Use(
		logger(),
	)
	if exposeMetrics {
		r.Use(metrics.PrometheusMiddleware())
		r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.PromRegistry, promhttp.HandlerOpts{})))
	}
	r = buildEdgeAPI(r, engine)
	r.GET("/api/v1/ping", ping)

	return r
}

// ping is an endpoint to check if the server is up.
func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}

// buildEdgeAPI sets up the API which handles delta requests.
func buildEdgeAPI(r *gin.Engine, engine dorasengine.Engine) *gin.Engine {
	log.Debug("Building edge API")
	edgeApiPath, err := url.JoinPath("/", apicommon.ApiBasePathV1, apicommon.DeltaApiPath)
	if err != nil {
		log.Error(err)
		panic(err)
	}
	edgeAPI := r.Group(edgeApiPath)
	edgeAPI.GET("/", func(c *gin.Context) {
		apiDelegate := gindelegate.NewDelegate(c)
		metrics.DeltaRequestCounter.Inc()
		engine.HandleReadDelta(apiDelegate)
	})
	return r
}
