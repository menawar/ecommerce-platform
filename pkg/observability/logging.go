// Package observability holds the cross-cutting telemetry helpers every service
// shares: structured logging now, Prometheus + OpenTelemetry later. It lives in
// pkg/ because it has zero business logic and zero service-specific knowledge —
// the dependency arrow points services -> observability, never the reverse.
package observability

import (
	"log/slog"
	"os"
)

// NewLogger returns a JSON structured logger tagged with the service name.
//
// We emit JSON (not the default text handler) because in production logs are
// shipped to a system that indexes fields — "service", "addr", "err" become
// queryable keys, not substrings to regex out of a sentence. The `service`
// attribute is attached once here via With(), so every line this logger (and
// its children) produces is automatically attributable to one service.
func NewLogger(service string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler).With("service", service)
}
