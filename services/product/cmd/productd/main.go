// Command productd runs the Product Catalog service: the product.v1.ProductService
// gRPC server (backed by a pgx pool) plus an HTTP server for /metrics and /healthz,
// under one errgroup with graceful shutdown. Same skeleton as userd; the new piece
// is the Postgres connection pool, opened at startup and closed on shutdown.
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

	"github.com/menawar/ecommerce-platform/pkg/grpcmw"
	"github.com/menawar/ecommerce-platform/pkg/observability"
	"github.com/menawar/ecommerce-platform/pkg/postgres"
	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	"github.com/menawar/ecommerce-platform/services/product/internal/server"
)

type config struct {
	grpcAddr string
	httpAddr string
	dbURL    string
}

func main() {
	log := observability.NewLogger("product")
	cfg := config{
		grpcAddr: env("PRODUCT_GRPC_ADDR", ":50052"),
		httpAddr: env("PRODUCT_HTTP_ADDR", ":2113"),
		dbURL:    env("PRODUCT_DB_URL", "postgres://ecommerce:ecommerce@localhost:5433/productdb?sslmode=disable"),
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
	// Open the pool first and fail fast if the DB is unreachable — a product
	// service with no database can't do anything useful.
	pool, err := postgres.NewPool(ctx, cfg.dbURL)
	if err != nil {
		return fmt.Errorf("connect productdb: %w", err)
	}
	defer pool.Close()
	log.Info("connected to productdb")

	metrics := grpcmw.NewMetrics(prometheus.DefaultRegisterer, "product")
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcmw.UnaryLogging(log),
			grpcmw.UnaryMetrics(metrics),
			grpcmw.UnaryRecovery(log),
		),
	)
	productv1.RegisterProductServiceServer(grpcServer, server.NewServer(pool, log))
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
