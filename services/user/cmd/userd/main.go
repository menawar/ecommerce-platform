// Command userd runs the User service: the user.v1.UserService gRPC server plus
// an HTTP server for /metrics and /healthz, side by side under one errgroup with
// graceful shutdown — the same skeleton as the Phase 0 hello service. The new
// material here is 12-factor configuration: everything that varies between
// environments (ports, the JWT signing secret) comes from the environment, not
// from code.
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
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/menawar/ecommerce-platform/pkg/auth"
	"github.com/menawar/ecommerce-platform/pkg/grpcmw"
	"github.com/menawar/ecommerce-platform/pkg/observability"
	"github.com/menawar/ecommerce-platform/pkg/postgres"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/user/internal/server"
	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

// config is the resolved runtime configuration. Gathering it into one struct
// (rather than calling os.Getenv scattered through the code) makes the service's
// full set of knobs visible in one place and easy to validate.
type config struct {
	grpcAddr   string
	httpAddr   string
	jwtSecret  string
	dbURL      string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func main() {
	log := observability.NewLogger("user")

	cfg := config{
		grpcAddr:   env("USER_GRPC_ADDR", ":50051"),
		httpAddr:   env("USER_HTTP_ADDR", ":2112"),
		jwtSecret:  os.Getenv("JWT_SECRET"),
		dbURL:      env("USER_DB_URL", "postgres://ecommerce:ecommerce@localhost:5433/userdb?sslmode=disable"),
		accessTTL:  15 * time.Minute,
		refreshTTL: 7 * 24 * time.Hour,
	}
	if cfg.jwtSecret == "" {
		// Fail soft in dev, but make the insecurity LOUD. Never let an unset
		// secret pass silently — a predictable secret means forgeable tokens.
		cfg.jwtSecret = "dev-insecure-secret-change-me"
		log.Warn("JWT_SECRET not set — using an insecure dev default; DO NOT use in production")
	}

	// Cancelled on SIGINT/SIGTERM; drives the whole shutdown (see hellod).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log, cfg); err != nil {
		log.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	log.Info("server stopped cleanly")
}

func run(ctx context.Context, log *slog.Logger, cfg config) error {
	// Compose the service. The repo is now Postgres-backed — the ONE line that
	// changed from the in-memory version (everything downstream depends on the
	// store.Repository interface, so the server is untouched).
	pool, err := postgres.NewPool(ctx, cfg.dbURL)
	if err != nil {
		return fmt.Errorf("connect userdb: %w", err)
	}
	defer pool.Close()
	log.Info("connected to userdb")

	repo := store.NewPostgres(pool)
	accessMgr := auth.NewJWTManager(cfg.jwtSecret, cfg.accessTTL)
	refreshMgr := auth.NewJWTManager(cfg.jwtSecret, cfg.refreshTTL)
	userSrv := server.NewServer(repo, accessMgr, refreshMgr, accessMgr, log)

	// Interceptors run in chain order: logging (outer) wraps recovery (inner) wraps
	// the handler — so logging records the final status even when recovery has
	// turned a panic into an Internal error.
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcmw.UnaryLogging(log),
			grpcmw.UnaryRecovery(log),
		),
	)
	userv1.RegisterUserServiceServer(grpcServer, userSrv)
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

// env returns the value of key, or def when unset/empty.
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
