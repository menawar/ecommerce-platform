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
