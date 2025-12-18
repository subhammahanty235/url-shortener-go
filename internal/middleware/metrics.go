package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/subhammahanty235/url-shortener/internal/pkg/metrics"
)

// MetricsMiddleware tracks HTTP request metrics for observability
// This middleware automatically instruments all HTTP endpoints
//
// Metrics tracked:
// 1. Request count (by endpoint, method, status)
// 2. Request duration (histogram for P50/P95/P99 calculations)
// 3. Active requests (current in-flight requests)
//
// How it works:
// - Before handler: Start timer, increment active requests
// - After handler: Record duration, increment counter, decrement active requests
func MetricsMiddleware(m *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start tracking time for this request
		start := time.Now()

		// Increment active requests gauge
		// Why? This shows saturation - if it keeps growing, you're overloaded
		m.HTTPRequestsActive.Inc()

		// Ensure we decrement active requests when done (even if panic occurs)
		defer m.HTTPRequestsActive.Dec()

		// Get the route path (e.g., "/api/v1/shorten" not "/api/v1/shorten?foo=bar")
		// Why use FullPath()? Groups all requests to same endpoint together
		// Without this, /user/123 and /user/456 would be separate metrics
		path := c.FullPath()
		if path == "" {
			// For 404s or unmatched routes, use the raw path
			path = c.Request.URL.Path
		}

		// Process the request
		c.Next()

		// Calculate request duration
		duration := time.Since(start).Seconds()

		// Get HTTP method and status code
		method := c.Request.Method
		status := strconv.Itoa(c.Writer.Status())

		// Record metrics after request completes

		// 1. Increment request counter
		// Labels: endpoint, method, status
		// PromQL to calculate error rate: rate(http_requests_total{status=~"5.."}[5m])
		m.HTTPRequestsTotal.WithLabelValues(path, method, status).Inc()

		// 2. Observe request duration in histogram
		// This stores the duration in appropriate bucket
		// PromQL for P95: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
		m.HTTPRequestDuration.WithLabelValues(path, method).Observe(duration)

		// Learning: Why observe AFTER c.Next()?
		// - c.Next() blocks until handler completes
		// - We want to measure total request time including all middleware
		// - If we observe before, we'd only measure middleware overhead
	}
}

// Key Learning: Middleware Order Matters!
//
// Correct order in main.go:
// 1. Recovery middleware (catches panics)
// 2. Logging middleware (logs all requests)
// 3. Metrics middleware (measures everything)
// 4. Authentication middleware (may reject requests)
// 5. Your handlers
//
// Why? If metrics is first, you'd miss logging. If auth is first, you miss measuring rejected requests.

// Advanced: Custom Metric Labels
//
// You can add more labels for finer granularity:
// - user_id (which users are making most requests?)
// - region (which region is slowest?)
// - version (is new version slower?)
//
// BUT: High cardinality kills Prometheus!
// - Bad: user_id (millions of unique values = millions of time series)
// - Good: user_type (free, premium, enterprise = 3 time series)
//
// Rule of thumb: Keep unique label combinations under 1000
