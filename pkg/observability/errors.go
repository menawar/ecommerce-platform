package observability

import (
	"context"
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
	"go.opentelemetry.io/otel/trace"
)

// Reporter sends unexpected errors to an external error-tracking sink (Sentry).
// It is the seam that keeps the rest of the code Sentry-agnostic: handlers and
// interceptors call Report; whether that reaches Sentry or nothing is a wiring
// decision. Structured logging already happens in the logging interceptor, so the
// default (no DSN) Reporter is a no-op rather than a second log line.
type Reporter interface {
	Report(ctx context.Context, err error, tags map[string]string)
	Close()
}

// NewReporter returns a Sentry-backed Reporter when dsn is set, else a no-op. This
// is the config gate: unset SENTRY_DSN (dev/CI) => nothing is sent; set it (prod)
// => real Sentry. Matches the MockProvider/SMTP-vs-Mailpit pattern.
func NewReporter(dsn, service, environment string, log *slog.Logger) Reporter {
	if dsn == "" {
		return noopReporter{}
	}
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Environment: environment,
		ServerName:  service,
	}); err != nil {
		// A bad DSN must not stop the service from starting — degrade to no-op.
		log.Error("sentry init failed; error reporting disabled", "err", err)
		return noopReporter{}
	}
	log.Info("error reporting enabled (sentry)", "service", service, "environment", environment)
	return &sentryReporter{service: service}
}

type noopReporter struct{}

func (noopReporter) Report(context.Context, error, map[string]string) {}
func (noopReporter) Close()                                           {}

type sentryReporter struct{ service string }

func (r *sentryReporter) Report(ctx context.Context, err error, tags map[string]string) {
	if err == nil {
		return
	}
	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("service", r.service)
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		// Correlate the Sentry event with the distributed trace (Jaeger/OTel) so an
		// incident can jump from the error to the full RPC trace.
		if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
			scope.SetTag("trace_id", sc.TraceID().String())
		}
	})
	hub.CaptureException(err)
}

// Close flushes buffered events on shutdown so in-flight reports aren't lost.
func (r *sentryReporter) Close() { sentry.Flush(2 * time.Second) }
