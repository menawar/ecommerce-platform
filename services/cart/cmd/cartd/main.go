// Command cartd runs the Cart service: the cart.v1.CartService gRPC server backed
// by Redis, plus an HTTP server for /metrics and /healthz. Same skeleton as the
// other daemons; the new piece is the Redis connection.
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

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/menawar/ecommerce-platform/pkg/grpcmw"
	"github.com/menawar/ecommerce-platform/pkg/observability"
	cartv1 "github.com/menawar/ecommerce-platform/proto/cart/v1"
	"github.com/menawar/ecommerce-platform/services/cart/internal/server"
	"github.com/menawar/ecommerce-platform/services/cart/internal/store"
)

type config struct {
	grpcAddr string
	httpAddr string
	redisURL string
}

func main() {
	log := observability.NewLogger("cart")
	cfg := config{
		grpcAddr: env("CART_GRPC_ADDR", ":50053"),
		httpAddr: env("CART_HTTP_ADDR", ":2114"),
		redisURL: env("REDIS_URL", "redis://localhost:6379/0"),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log, cfg); err != nil {
		log.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	log.Info("server stopped cleanly")
}

func run(ctx context.Context, log *slog.Logger, cfg config) error {
	opts, err := redis.ParseURL(cfg.redisURL)
	if err != nil {
		return fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(opts)
	defer func() { _ = client.Close() }()

	// Ping so we fail fast if Redis is unreachable, rather than on first request.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	log.Info("connected to redis")

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcmw.UnaryLogging(log),
			grpcmw.UnaryRecovery(log),
		),
	)
	cartv1.RegisterCartServiceServer(grpcServer, server.NewServer(store.NewRedis(client), log))
	reflection.Register(grpcServer)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	httpServer := &http.Server{Addr: cfg.httpAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		lis, err := net.Listen("tcp", cfg.grpcAddr)
		if err != nil {
			return fmt.Errorf("grpc listen: %w", err)
		}
		log.Info("grpc server listening", "addr", cfg.grpcAddr)
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("grpc serve: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		log.Info("http server listening", "addr", cfg.httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http serve: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		log.Info("shutdown requested, draining servers")
		grpcServer.GracefulStop()
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
