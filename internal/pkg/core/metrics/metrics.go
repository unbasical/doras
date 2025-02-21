package metrics

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"strconv"
	"time"
)

var (
	PromRegistry        = prometheus.NewRegistry()
	DorasRegisterer     = prometheus.WrapRegistererWithPrefix("doras_", PromRegistry)
	DeltaRequestCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "delta_requests_total",
			Help: "Total number of inbound delta requests",
		},
	)
	ExpiredDummiesCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "expired_dummies_total",
			Help: "Total number of expired dummies",
		},
	)
	DeltaCreationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "delta_creation_duration_seconds",
			Help:    "Duration of delta creation in seconds",
			Buckets: prometheus.ExponentialBuckets(0.5, 4, 12),
		},
		[]string{"diff_algo", "comp_algo", "success"},
	)
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests processed.",
		},
		[]string{"code", "method", "path"},
	)
	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"code", "method", "path"},
	)
)

func init() {
	// register collectors
	DorasRegisterer.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	DorasRegisterer.MustRegister(collectors.NewGoCollector())
	// register universal metrics
	DorasRegisterer.MustRegister(DeltaRequestCounter)
	DorasRegisterer.MustRegister(ExpiredDummiesCounter)
	DorasRegisterer.MustRegister(DeltaCreationDuration)
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

		HttpRequestsTotal.With(prometheus.Labels{
			"code":   strconv.Itoa(statusCode),
			"method": method,
			"path":   path,
		}).Inc()

		HttpRequestDuration.With(prometheus.Labels{
			"code":   strconv.Itoa(statusCode),
			"method": method,
			"path":   path,
		}).Observe(duration)
	}
}
