package api

import (
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/unbasical/doras/internal/pkg/api/gindelegate"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/unbasical/doras/internal/pkg/api/apicommon"
	"github.com/unbasical/doras/internal/pkg/core/dorasengine"
)

var (
	registry            = prometheus.NewRegistry()
	deltaRequestCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "delta_requests_total",
			Help: "Total number of inbound delta requests",
		},
	)
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests processed.",
		},
		[]string{"code", "method", "path"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"code", "method", "path"},
	)
)

func init() {
	prefixedRegisterer := prometheus.WrapRegistererWithPrefix("doras_", registry)
	prefixedRegisterer.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	prefixedRegisterer.MustRegister(collectors.NewGoCollector())
	prefixedRegisterer.MustRegister(deltaRequestCounter)
	prefixedRegisterer.MustRegister(httpRequestsTotal)
	prefixedRegisterer.MustRegister(httpRequestDuration)
}

// PrometheusMiddleware is a Gin middleware that instruments HTTP requests.
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		c.Next()

		statusCode := c.Writer.Status()
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		duration := time.Since(startTime).Seconds()

		httpRequestsTotal.With(prometheus.Labels{
			"code":   strconv.Itoa(statusCode),
			"method": method,
			"path":   path,
		}).Inc()

		httpRequestDuration.With(prometheus.Labels{
			"code":   strconv.Itoa(statusCode),
			"method": method,
			"path":   path,
		}).Observe(duration)
	}
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
		r.Use(PrometheusMiddleware())
		r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(registry, promhttp.HandlerOpts{})))
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
		deltaRequestCounter.Inc()
		engine.HandleReadDelta(apiDelegate)
	})
	return r
}
