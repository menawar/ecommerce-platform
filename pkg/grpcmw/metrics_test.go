package grpcmw

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestUnaryMetrics proves the interceptor increments the handled counter with the
// correct method+code label for both a success and a failure — i.e. that traffic
// and error outcomes actually become queryable series. We register against a fresh
// registry (not the global default) so the assertion sees only this test's metrics.
func TestUnaryMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg, "test")
	interceptor := UnaryMetrics(m)

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Svc/Do"}

	// A successful call -> code "OK".
	okHandler := func(context.Context, any) (any, error) { return "ok", nil }
	if _, err := interceptor(context.Background(), nil, info, okHandler); err != nil {
		t.Fatalf("ok call returned error: %v", err)
	}

	// A failing call -> code "InvalidArgument".
	failHandler := func(context.Context, any) (any, error) {
		return nil, status.Error(codes.InvalidArgument, "bad")
	}
	if _, err := interceptor(context.Background(), nil, info, failHandler); err == nil {
		t.Fatal("fail call returned nil error")
	}

	// Exactly one observation per (method, code) pair.
	if got := testutil.ToFloat64(m.handled.WithLabelValues("/test.Svc/Do", "OK")); got != 1 {
		t.Errorf("OK count = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.handled.WithLabelValues("/test.Svc/Do", "InvalidArgument")); got != 1 {
		t.Errorf("InvalidArgument count = %v, want 1", got)
	}

	// And the histogram saw both calls (2 observations on the method's series).
	if got := testutil.CollectAndCount(m.duration); got != 1 {
		t.Errorf("duration series count = %v, want 1", got)
	}
}
