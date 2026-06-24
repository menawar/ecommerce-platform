// Package httputil provides reusable HTTP middleware for any chi-based server.
// It mirrors the gRPC interceptor pattern from pkg/grpcmw but adapted for HTTP
// dimensions: method (GET/POST/…), route pattern, and status code.
package httputil

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPMetrics holds the Prometheus collectors for HTTP server traffic. Same
// constructor-injection pattern as grpcmw.Metrics: the caller passes a
// Registerer (the global default in production, a fresh registry in tests).
type HTTPMetrics struct {
	handled  *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewHTTPMetrics creates and registers HTTP request collectors. The constant
// "service" label lets a multi-service Prometheus disambiguate (e.g.
// service="gateway") — identical to what we do on the gRPC side.
//
// Label design matters for cardinality:
//   - method:  bounded (GET, POST, PUT, DELETE, …) — safe.
//   - pattern: the chi ROUTE PATTERN (e.g. "/products/{id}"), NOT the raw URL.
//     Raw URLs would create a new time series per unique product ID — an
//     unbounded cardinality explosion that kills Prometheus. Using the route
//     pattern collapses all /products/abc, /products/xyz into one series.
//   - code:    bounded (a few dozen HTTP status codes) — safe.
func NewHTTPMetrics(reg prometheus.Registerer, service string) *HTTPMetrics {
	factory := promauto.With(reg)
	constLabels := prometheus.Labels{"service": service}
	return &HTTPMetrics{
		handled: factory.NewCounterVec(prometheus.CounterOpts{
			Name:        "http_server_handled_total",
			Help:        "Total HTTP requests completed, by method, route pattern, and status code.",
			ConstLabels: constLabels,
		}, []string{"method", "pattern", "code"}),
		duration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "http_server_handling_seconds",
			Help:        "HTTP request handling latency in seconds, by method and route pattern.",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: constLabels,
		}, []string{"method", "pattern"}),
	}
}

// Middleware returns a chi-compatible middleware that records a count + latency
// observation for every HTTP request. It MUST be placed AFTER chi's routing has
// resolved — that's the only point where chi.RouteContext contains the matched
// pattern. In practice, using r.Use() on the root router is fine because chi
// resolves the route BEFORE calling the middleware chain's inner handler.
//
// The key technique: chi's middleware.NewWrapResponseWriter intercepts the
// WriteHeader call, letting us read back the status code after the handler
// returns. A bare http.ResponseWriter doesn't expose Status() — that's the
// whole reason the wrapper exists. We already use this in the gateway's logging
// middleware, so the concept is familiar.
func Middleware(m *HTTPMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap so we can read back the status code that the handler wrote.
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			// chi resolves the route pattern BEFORE calling the handler chain,
			// so by the time we get here RouteContext is populated. If no route
			// matched (404), the pattern is empty — we use "unmatched" to keep
			// the label value non-empty and make it searchable.
			pattern := "unmatched"
			if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
				pattern = rctx.RoutePattern()
			}

			code := strconv.Itoa(ww.Status())
			m.handled.WithLabelValues(r.Method, pattern, code).Inc()
			m.duration.WithLabelValues(r.Method, pattern).Observe(time.Since(start).Seconds())
		})
	}
}
