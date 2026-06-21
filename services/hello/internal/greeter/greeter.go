// Package greeter implements the throwaway GreeterService. It lives under
// internal/ so NOTHING outside this service module can import it: the package
// is an implementation detail, not part of any contract. The contract is the
// generated proto; the implementation is private.
package greeter

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	hellov1 "github.com/menawar/ecommerce-platform/proto/hello/v1"
)

// Server implements the generated hellov1.GreeterServiceServer interface.
//
// Embedding UnimplementedGreeterServiceServer is REQUIRED by protoc-gen-go-grpc
// (its forward-compat default): it gives us a default impl for every RPC, so if
// the .proto later adds an RPC we haven't written yet, this still compiles and
// returns codes.Unimplemented instead of breaking the build.
type Server struct {
	hellov1.UnimplementedGreeterServiceServer

	log       *slog.Logger
	greetings prometheus.Counter
}

// NewServer wires the dependencies. We ACCEPT a prometheus.Registerer (an
// interface) rather than reaching for the global default registry: production
// code passes prometheus.DefaultRegisterer, but a test passes a fresh
// prometheus.NewRegistry() so each test gets its own clean metrics and two
// Servers never collide on duplicate registration. Accept-interfaces is what
// makes this constructor testable.
func NewServer(log *slog.Logger, reg prometheus.Registerer) *Server {
	return &Server{
		log: log,
		greetings: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: "hello_greetings_total",
			Help: "Total number of SayHello requests served.",
		}),
	}
}

// SayHello is the one RPC. The first arg is always context.Context: gRPC fills
// it with the call's deadline and cancellation, so a slow/abandoned client
// frees server work. We thread it into the log call for the same reason.
func (s *Server) SayHello(ctx context.Context, req *hellov1.SayHelloRequest) (*hellov1.SayHelloResponse, error) {
	// Always use the generated GetX() accessors, not req.Name directly: they're
	// nil-safe and survive proto field changes (e.g. a field moving into a oneof).
	name := req.GetName()
	if name == "" {
		name = "world"
	}

	s.greetings.Inc()
	s.log.InfoContext(ctx, "handled SayHello", "name", name)

	return &hellov1.SayHelloResponse{Message: "hello, " + name}, nil
}
