package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "juke_http_requests_total",
		Help: "Total HTTP requests handled by the API",
	}, []string{"method", "path", "status"})
	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "juke_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

// Middleware records request counts and latency for Prometheus.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		status := strconv.Itoa(c.Writer.Status())
		httpRequests.WithLabelValues(c.Request.Method, path, status).Inc()
		httpDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}

// Register mounts GET /metrics on the Gin engine.
func Register(r *gin.Engine) {
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
