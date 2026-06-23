// Command notificationd runs the Notification service: a PURE event consumer. It
// exposes no gRPC business API — only /healthz and /metrics — and processes events
// off NATS JetStream idempotently into notificationdb.
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

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"

	"github.com/menawar/ecommerce-platform/pkg/events"
	"github.com/menawar/ecommerce-platform/pkg/observability"
	"github.com/menawar/ecommerce-platform/pkg/postgres"
	"github.com/menawar/ecommerce-platform/services/notification/internal/notify"
)

type config struct {
	httpAddr string
	dbURL    string
	natsURL  string
}

func main() {
	log := observability.NewLogger("notification")
	cfg := config{
		httpAddr: env("NOTIFICATION_HTTP_ADDR", ":2117"),
		dbURL:    env("NOTIFICATION_DB_URL", "postgres://ecommerce:ecommerce@localhost:5433/notificationdb?sslmode=disable"),
		natsURL:  env("NATS_URL", "nats://localhost:4222"),
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
	pool, err := postgres.NewPool(ctx, cfg.dbURL)
	if err != nil {
		return fmt.Errorf("connect notificationdb: %w", err)
	}
	defer pool.Close()

	nc, js, err := events.Connect(ctx, cfg.natsURL, events.StreamName, events.StreamSubjects())
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}
	defer nc.Close()
	log.Info("connected to notificationdb and nats")

	handler := notify.NewHandler(pool, notify.LogSender{Log: log}, log)
	// Start the durable consumer. It runs in its own goroutines; we stop it on
	// shutdown.
	cc, err := events.Consume(ctx, js, events.StreamName, "notification", log, handler.Handle)
	if err != nil {
		return fmt.Errorf("start consumer: %w", err)
	}
	defer cc.Stop()
	log.Info("notification consumer started")

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	httpServer := &http.Server{Addr: cfg.httpAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		log.Info("http server listening", "addr", cfg.httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http serve: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()
		log.Info("shutdown requested, draining")
		cc.Stop()
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
