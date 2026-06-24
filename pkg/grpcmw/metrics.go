package grpcmw

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Metrics holds the Prometheus collectors for gRPC server traffic. One instance
// per service process, constructed once at startup and shared by the interceptor.
type Metrics struct {
	handled  *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewMetrics builds and registers the gRPC server collectors against reg. The
// `service` name becomes a CONSTANT label on every series, so a single Prometheus
// can tell user-service latency from order-service latency even though both
// export the same metric names.
//
// We take an injected Registerer (rather than reaching for the global default
// registry inside) for two reasons: a test can pass a fresh prometheus.NewRegistry()
// and assert on this service's metrics in isolation, and a duplicate registration
// surfaces as a returned/observable error instead of a hidden global panic. In
// production main() passes prometheus.DefaultRegisterer, which is what promhttp's
// /metrics handler serves.
func NewMetrics(reg prometheus.Registerer, service string) *Metrics {
	factory := promauto.With(reg)
	constLabels := prometheus.Labels{"service": service}
	return &Metrics{
		// A COUNTER (monotonic, only goes up): the `_total` suffix is the Prometheus
		// convention. method+code labels let PromQL derive QPS (rate over the counter)
		// and error rate (the share of series where code != "OK").
		handled: factory.NewCounterVec(prometheus.CounterOpts{
			Name:        "grpc_server_handled_total",
			Help:        "Total gRPC requests completed, by method and status code.",
			ConstLabels: constLabels,
		}, []string{"method", "code"}),
		// A HISTOGRAM (not a summary): it pre-buckets observations, and buckets are
		// addable across instances — so you can compute a fleet-wide p99 in Prometheus.
		// A summary computes quantiles client-side and CANNOT be aggregated. DefBuckets
		// are in seconds (.005–10), a sensible default for RPC latency. We deliberately
		// omit a `code` label here to keep cardinality (buckets × methods) bounded.
		duration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "grpc_server_handling_seconds",
			Help:        "gRPC request handling latency in seconds, by method.",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: constLabels,
		}, []string{"method"}),
	}
}

// UnaryMetrics records a count + latency observation for every unary RPC. Like
// UnaryLogging it observes the FINAL outcome, so place it OUTER of UnaryRecovery
// in the chain: a panic that recovery converts into codes.Internal is then counted
// as an Internal failure rather than silently dropped.
func UnaryMetrics(m *Metrics) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err) // OK when err == nil

		m.handled.WithLabelValues(info.FullMethod, code.String()).Inc()
		m.duration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
		return resp, err
	}
}
