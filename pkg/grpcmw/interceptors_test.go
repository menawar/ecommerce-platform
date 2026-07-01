package grpcmw_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/menawar/ecommerce-platform/pkg/grpcmw"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

var info = &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/Login"}

// TestUnaryRecovery_Panic proves a panicking handler becomes codes.Internal and
// does NOT propagate (the test itself would crash if it did).
func TestUnaryRecovery_Panic(t *testing.T) {
	interceptor := grpcmw.UnaryRecovery(discardLogger())

	_, err := interceptor(context.Background(), nil, info, func(context.Context, any) (any, error) {
		panic("boom")
	})

	if status.Code(err) != codes.Internal {
		t.Fatalf("want codes.Internal after panic, got %v", err)
	}
}

// TestUnaryRecovery_PassThrough proves a normal handler's result is untouched.
func TestUnaryRecovery_PassThrough(t *testing.T) {
	interceptor := grpcmw.UnaryRecovery(discardLogger())

	resp, err := interceptor(context.Background(), nil, info, func(context.Context, any) (any, error) {
		return "ok", nil
	})
	if err != nil || resp != "ok" {
		t.Fatalf("pass-through altered result: resp=%v err=%v", resp, err)
	}
}

// TestUnaryLogging emits one line carrying the method and status code, and passes
// the handler's result through unchanged.
func TestUnaryLogging(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	interceptor := grpcmw.UnaryLogging(log)

	resp, err := interceptor(context.Background(), nil, info, func(context.Context, any) (any, error) {
		return nil, status.Error(codes.Unauthenticated, "nope")
	})

	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("logging altered the error: %v", err)
	}
	if resp != nil {
		t.Errorf("logging altered the response: %v", resp)
	}

	line := buf.String()
	if !strings.Contains(line, "/user.v1.UserService/Login") {
		t.Errorf("log missing method: %s", line)
	}
	if !strings.Contains(line, "Unauthenticated") {
		t.Errorf("log missing status code: %s", line)
	}
}

// captureReporter records what UnaryErrorReporting sends to the error sink.
type captureReporter struct {
	calls int
	last  error
	tags  map[string]string
}

func (c *captureReporter) Report(_ context.Context, err error, tags map[string]string) {
	c.calls++
	c.last = err
	c.tags = tags
}

func TestUnaryErrorReporting(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantReport bool
	}{
		{"success", nil, false},
		{"internal is our fault", status.Error(codes.Internal, "boom"), true},
		{"unknown is our fault", status.Error(codes.Unknown, "?"), true},
		{"invalid arg is the client's fault", status.Error(codes.InvalidArgument, "bad"), false},
		{"not found is not reported", status.Error(codes.NotFound, "nope"), false},
		{"unavailable is transient, not reported", status.Error(codes.Unavailable, "down"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := &captureReporter{}
			interceptor := grpcmw.UnaryErrorReporting(rep)
			_, err := interceptor(context.Background(), nil, info, func(context.Context, any) (any, error) {
				return nil, tc.err
			})
			if status.Code(err) != status.Code(tc.err) {
				t.Errorf("interceptor changed the error: %v", err)
			}
			if got := rep.calls > 0; got != tc.wantReport {
				t.Errorf("reported=%v, want %v", got, tc.wantReport)
			}
			if tc.wantReport && rep.tags["method"] != info.FullMethod {
				t.Errorf("missing method tag: %v", rep.tags)
			}
		})
	}
}
