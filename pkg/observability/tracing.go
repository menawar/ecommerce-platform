package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTracer bootstraps OpenTelemetry tracing for a single service process.
// It creates an OTLP gRPC exporter that sends spans to the collector at
// otlpEndpoint (e.g. "localhost:4317" — Jaeger's OTLP ingestion port).
//
// Design decisions:
//
//   - We set the GLOBAL TracerProvider and TextMapPropagator. OTel instrumentation
//     libraries (otelgrpc, otelhttp) read these globals, so a single InitTracer
//     call wires up every instrumented gRPC/HTTP call in the process. This is
//     the intended use: OTel's global package exists exactly for this pattern.
//
//   - We use W3C Trace Context propagation (traceparent/tracestate headers).
//     This is the internet standard and the default for OTel, so it works
//     across gRPC metadata and HTTP headers without any custom plumbing.
//
//   - BatchSpanProcessor is used (the default from NewTracerProvider) — it
//     buffers spans and exports in batches for performance. In production
//     you'd tune BatchTimeout/MaxExportBatchSize; defaults are fine for dev.
//
//   - The returned shutdown function MUST be called (via defer) to flush any
//     buffered spans before the process exits. Without it, the last few spans
//     of a graceful shutdown are silently dropped.
func InitTracer(ctx context.Context, service, otlpEndpoint string) (shutdown func(context.Context) error, err error) {
	// The exporter serialises spans as OTLP protobuf and sends them over gRPC
	// to the collector. WithInsecure() is fine inside the Docker Compose network
	// (no TLS); in production you'd use TLS credentials.
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}

	// A Resource describes WHAT is producing spans — the service name appears
	// in Jaeger's service dropdown and lets you filter by service.
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(service),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set globals so otelgrpc/otelhttp pick up this provider automatically.
	otel.SetTracerProvider(tp)

	// W3C Trace Context: propagates trace_id + span_id across process boundaries
	// via standard headers (traceparent, tracestate). When the gateway starts a
	// span and then calls the order service via gRPC, otelgrpc reads this
	// propagator and injects traceparent into gRPC metadata. The order service's
	// otelgrpc server interceptor extracts it and creates a CHILD span under the
	// same trace — that's what makes a single trace span multiple services.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp.Shutdown, nil
}
