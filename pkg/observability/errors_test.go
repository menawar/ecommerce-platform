package observability_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/menawar/ecommerce-platform/pkg/observability"
)

// With no DSN, NewReporter returns a no-op that is safe to Report/Close (the
// dev/CI default — nothing is sent, nothing panics).
func TestNewReporter_NoDSNIsNoop(t *testing.T) {
	r := observability.NewReporter("", "test", "development", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if r == nil {
		t.Fatal("expected a non-nil no-op reporter")
	}
	r.Report(context.Background(), errors.New("boom"), map[string]string{"k": "v"})
	r.Report(context.Background(), nil, nil)
	r.Close()
}
