// Command hellod is the throwaway hello daemon: it runs a gRPC server (the
// GreeterService) and an HTTP server (/metrics, /healthz) SIDE BY SIDE in one
// process, and shuts both down cleanly on SIGINT/SIGTERM. This is the Phase 0
// skeleton every real service will copy: two listeners, one errgroup, one
// context-driven shutdown.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/menawar/ecommerce-platform/pkg/observability"
	hellov1 "github.com/menawar/ecommerce-platform/proto/hello/v1"
	"github.com/menawar/ecommerce-platform/services/hello/internal/greeter"
)

const (
	grpcAddr = ":50051" // gRPC clients connect here
	httpAddr = ":2112"  // Prometheus scrapes /metrics here (9100 is taken by node_exporter)
)

func main() {
	log := observability.NewLogger("hello")

	// signal.NotifyContext returns a context that is cancelled the moment the
	// process receives SIGINT (Ctrl-C) or SIGTERM (docker stop / k8s). This is
	// the SINGLE source of "time to shut down" — we never poll a flag or read a
	// channel by hand; cancellation propagates through ctx to every goroutine.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log); err != nil {
		log.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	log.Info("server stopped cleanly")
}

// run owns the lifecycle. We keep it separate from main() so it can return an
// error (main can't be tested; a function returning error can) and so all
// deferred cleanup runs before os.Exit.
func run(ctx context.Context, log *slog.Logger) error {
	grpcServer := grpc.NewServer()
	hellov1.RegisterGreeterServiceServer(grpcServer, greeter.NewServer(log, prometheus.DefaultRegisterer))
	// Server reflection lets tools like grpcurl discover our services/methods at
	// runtime without the .proto file. Convenient in dev; we'll gate it off in
	// prod later.
	reflection.Register(grpcServer)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
		// Guards against a slow-loris client holding a connection open forever
		// while sending headers a byte at a time. Cheap, always worth setting.
		ReadHeaderTimeout: 5 * time.Second,
	}

	// errgroup.WithContext gives us a derived ctx that is cancelled when EITHER
	// the parent ctx is cancelled (a signal) OR any g.Go func returns a non-nil
	// error. So a fatal error in one listener triggers shutdown of the other —
	// the two servers live and die together.
	g, ctx := errgroup.WithContext(ctx)

	// --- gRPC listener ---
	g.Go(func() error {
		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			return fmt.Errorf("grpc listen: %w", err)
		}
		log.Info("grpc server listening", "addr", grpcAddr)
		// Serve blocks until GracefulStop/Stop is called; it then returns nil
		// (grpc.ErrServerStopped only on Serve-after-Stop), so a clean stop is
		// not an error.
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("grpc serve: %w", err)
		}
		return nil
	})

	// --- HTTP listener ---
	g.Go(func() error {
		log.Info("http server listening", "addr", httpAddr)
		// ListenAndServe returns ErrServerClosed on a clean Shutdown — expected,
		// not a failure. Anything else is a real error.
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http serve: %w", err)
		}
		return nil
	})

	// --- shutdown watcher ---
	// Blocks until ctx is cancelled (signal OR sibling error), then stops both
	// servers gracefully. GracefulStop drains in-flight RPCs; httpServer.Shutdown
	// drains in-flight requests, bounded by a fresh timeout context (the original
	// ctx is already cancelled, so we can't reuse it for the deadline).
	g.Go(func() error {
		<-ctx.Done()
		log.Info("shutdown requested, draining servers")
		grpcServer.GracefulStop()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	})

	// Wait blocks until ALL goroutines return, propagating the first error.
	return g.Wait()
}
