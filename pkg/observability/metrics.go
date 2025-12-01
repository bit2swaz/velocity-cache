package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	CacheOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vc_cache_operations_total",
		Help: "The total number of cache operations",
	}, []string{"operation", "result"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vc_http_duration_seconds",
		Help:    "Duration of HTTP requests.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "status"})

	ProxyTraffic = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vc_proxy_bytes_total",
		Help: "Total bytes transferred via the local proxy",
	}, []string{"direction"})
)
