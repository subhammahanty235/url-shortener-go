package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the application
// This pattern makes metrics easy to pass around and mock in tests
type Metrics struct {
	// HTTP Metrics (Application Layer)
	HTTPRequestsTotal   *prometheus.CounterVec   // Total requests by endpoint, method, status
	HTTPRequestDuration *prometheus.HistogramVec // Request latency by endpoint
	HTTPRequestsActive  prometheus.Gauge         // Currently in-flight requests

	// Business Metrics (Domain Layer)
	URLsCreatedTotal    prometheus.Counter       // Total URLs shortened
	URLRedirectsTotal   prometheus.Counter       // Total redirects served
	CustomAliasTotal    prometheus.Counter       // URLs created with custom aliases
	ExpiredURLsTotal    prometheus.Counter       // Expired URLs encountered

	// Cache Metrics (Infrastructure Layer)
	CacheHitsTotal   *prometheus.CounterVec // Cache hits by operation (get, set)
	CacheMissesTotal *prometheus.CounterVec // Cache misses by operation
	CacheErrors      *prometheus.CounterVec // Cache errors by operation

	// Database Metrics (Infrastructure Layer)
	DBQueryDuration *prometheus.HistogramVec // DB query duration by operation
	DBConnectionsActive prometheus.Gauge      // Active DB connections from pool
	DBErrors        *prometheus.CounterVec   // DB errors by operation
}

// NewMetrics creates and registers all Prometheus metrics
// Using promauto.New* functions automatically registers metrics with Prometheus
func NewMetrics() *Metrics {
	return &Metrics{
		// HTTP Request Counter
		// Labels: endpoint=/api/v1/shorten, method=POST, status=200
		// Use case: Track request volume and error rates per endpoint
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests by endpoint, method and status code",
			},
			[]string{"endpoint", "method", "status"},
		),

		// HTTP Request Duration Histogram
		// Buckets: 0.001s (1ms), 0.005s (5ms), 0.01s (10ms), ..., 10s
		// Use case: Calculate P50, P95, P99 latency for each endpoint
		// Why histogram? Allows Prometheus to calculate percentiles from buckets
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "http_request_duration_seconds",
				Help: "HTTP request latency in seconds (histogram for percentiles)",
				// Buckets define the boundaries for latency measurements
				// These are tuned for a URL shortener (should be fast!)
				Buckets: []float64{
					0.001, // 1ms   - ideal cache hit
					0.005, // 5ms   - good
					0.01,  // 10ms  - acceptable
					0.025, // 25ms  - slow cache hit
					0.05,  // 50ms  - DB query
					0.1,   // 100ms - slow DB query
					0.25,  // 250ms - very slow
					0.5,   // 500ms - concerning
					1.0,   // 1s    - bad
					2.5,   // 2.5s  - terrible
					5.0,   // 5s    - timeout territory
					10.0,  // 10s   - definitely timing out
				},
			},
			[]string{"endpoint", "method"},
		),

		// Active Requests Gauge
		// Use case: See current load, detect if requests are piling up (saturation)
		HTTPRequestsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_active",
				Help: "Number of HTTP requests currently being processed",
			},
		),

		// URLs Created Counter
		// Use case: Business metric - how many URLs are we shortening?
		URLsCreatedTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "urls_created_total",
				Help: "Total number of URLs shortened",
			},
		),

		// URL Redirects Counter
		// Use case: Business metric - how many clicks/redirects are happening?
		URLRedirectsTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "url_redirects_total",
				Help: "Total number of URL redirects served",
			},
		),

		// Custom Alias Counter
		// Use case: Track how many users use custom aliases vs auto-generated
		CustomAliasTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "custom_alias_total",
				Help: "Total number of URLs created with custom aliases",
			},
		),

		// Expired URLs Counter
		// Use case: Track how often users hit expired links (user experience metric)
		ExpiredURLsTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "expired_urls_total",
				Help: "Total number of expired URL access attempts",
			},
		),

		// Cache Hits Counter
		// Labels: operation=get_by_short_code
		// Use case: Calculate cache hit ratio = hits / (hits + misses)
		CacheHitsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "Total number of cache hits by operation",
			},
			[]string{"operation"},
		),

		// Cache Misses Counter
		// Use case: If misses are high, cache isn't effective (tune TTL or capacity)
		CacheMissesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "Total number of cache misses by operation",
			},
			[]string{"operation"},
		),

		// Cache Errors Counter
		// Use case: Track Redis connection issues
		CacheErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_errors_total",
				Help: "Total number of cache errors by operation",
			},
			[]string{"operation"},
		),

		// Database Query Duration Histogram
		// Labels: operation=create_url, get_by_short_code, etc.
		// Use case: Identify slow DB queries that need optimization
		DBQueryDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "db_query_duration_seconds",
				Help: "Database query duration in seconds by operation",
				// DB queries are generally slower than cache, so wider buckets
				Buckets: []float64{
					0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0,
				},
			},
			[]string{"operation"},
		),

		// Active DB Connections Gauge
		// Use case: Monitor connection pool saturation
		// If this equals max pool size, you're bottlenecked on DB connections
		DBConnectionsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "db_connections_active",
				Help: "Number of active database connections in the pool",
			},
		),

		// Database Errors Counter
		// Use case: Track DB failures (connection timeouts, query errors)
		DBErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "db_errors_total",
				Help: "Total number of database errors by operation",
			},
			[]string{"operation"},
		),
	}
}

// Key Learning: Metric Types Explained
//
// 1. Counter - Only goes up (resets on restart)
//    Examples: total requests, total errors, total URLs created
//    PromQL: rate(http_requests_total[5m]) gives requests/second over 5 min
//
// 2. Gauge - Can go up or down (current value)
//    Examples: active connections, memory usage, queue size
//    PromQL: avg_over_time(http_requests_active[5m]) gives average active requests
//
// 3. Histogram - Buckets of observations (for percentiles)
//    Examples: request duration, response size
//    PromQL: histogram_quantile(0.95, http_request_duration_seconds) gives P95
//    Stores: _bucket (count in each bucket), _sum (total), _count (observations)
//
// 4. Summary - Like histogram but calculates percentiles client-side (avoid in most cases)
//    Why avoid? Can't aggregate across instances, more expensive
//
// Best Practice: Use Counters and Histograms for most metrics. Use Gauges sparingly.
