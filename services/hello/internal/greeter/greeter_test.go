package greeter_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	hellov1 "github.com/menawar/ecommerce-platform/proto/hello/v1"
	"github.com/menawar/ecommerce-platform/services/hello/internal/greeter"
)

// discardLogger keeps test output clean — we assert on behavior, not log lines.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestSayHello is a table-driven unit test (Go's idiomatic test shape): one
// slice of cases, one loop, each case a named subtest. Adding a case is one
// line, and t.Run gives each its own name in the output.
//
// PROVES: the empty-name default ("world") and the basic greeting formatting.
func TestSayHello(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty name defaults to world", in: "", want: "hello, world"},
		{name: "uses provided name", in: "ada", want: "hello, ada"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Fresh registry per server => no duplicate-registration panic.
			s := greeter.NewServer(discardLogger(), prometheus.NewRegistry())

			got, err := s.SayHello(context.Background(), &hellov1.SayHelloRequest{Name: tc.in})
			if err != nil {
				t.Fatalf("SayHello returned error: %v", err)
			}
			if got.GetMessage() != tc.want {
				t.Errorf("message = %q, want %q", got.GetMessage(), tc.want)
			}
		})
	}
}

// TestSayHello_OverGRPC exercises the FULL gRPC path — marshal, transport,
// unmarshal — using bufconn, an in-memory net.Conn. No real TCP port, no
// flakiness, but the request genuinely travels through the generated client and
// server stubs. This is the automated proof of the Phase 0 acceptance criterion
// "hello service responds to a gRPC call".
func TestSayHello_OverGRPC(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)

	srv := grpc.NewServer()
	hellov1.RegisterGreeterServiceServer(srv, greeter.NewServer(discardLogger(), prometheus.NewRegistry()))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	// Dial the in-memory listener. The custom dialer hands back bufconn's
	// net.Conn instead of opening a socket; "passthrough" skips DNS resolution.
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := hellov1.NewGreeterServiceClient(conn)
	resp, err := client.SayHello(context.Background(), &hellov1.SayHelloRequest{Name: "grpc"})
	if err != nil {
		t.Fatalf("SayHello over gRPC: %v", err)
	}
	if want := "hello, grpc"; resp.GetMessage() != want {
		t.Errorf("message = %q, want %q", resp.GetMessage(), want)
	}
}
