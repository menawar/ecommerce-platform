// Command gatewayd runs the API gateway: an HTTP server that translates REST
// calls into gRPC calls to internal services. It holds a long-lived gRPC client
// connection to the User service and shuts down gracefully on a signal.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/menawar/ecommerce-platform/pkg/observability"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/gateway/internal/gateway"
)

func main() {
	log := observability.NewLogger("gateway")
	httpAddr := env("GATEWAY_HTTP_ADDR", ":8080")
	userAddr := env("USER_GRPC_ADDR", "localhost:50051")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log, httpAddr, userAddr); err != nil {
		log.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	log.Info("server stopped cleanly")
}

func run(ctx context.Context, log *slog.Logger, httpAddr, userAddr string) error {
	// grpc.NewClient creates a lazily-connecting client: it does NOT dial here,
	// it connects on the first RPC and reconnects automatically. So the gateway
	// can start before the user service is reachable. insecure creds = plaintext,
	// fine inside the trusted compose network; TLS would terminate at this edge.
	conn, err := grpc.NewClient(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create user client: %w", err)
	}
	defer func() { _ = conn.Close() }()

	h := gateway.NewHandler(userv1.NewUserServiceClient(conn), log)
	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           h.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Info("gateway listening", "addr", httpAddr, "user_grpc", userAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http serve: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		log.Info("shutdown requested, draining gateway")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	})

	return g.Wait()
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
