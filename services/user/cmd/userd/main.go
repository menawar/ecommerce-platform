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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/menawar/ecommerce-platform/pkg/auth"
	"github.com/menawar/ecommerce-platform/pkg/events"
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
	grpcAddr     string
	httpAddr     string
	jwtSecret    string
	dbURL        string
	natsURL      string
	otelEndpoint string
	webBaseURL   string
	accessTTL    time.Duration
	refreshTTL   time.Duration
}

func main() {
	log := observability.NewLogger("user")

	cfg := config{
		grpcAddr:     env("USER_GRPC_ADDR", ":50051"),
		httpAddr:     env("USER_HTTP_ADDR", ":2112"),
		jwtSecret:    os.Getenv("JWT_SECRET"),
		dbURL:        env("USER_DB_URL", "postgres://ecommerce:ecommerce@localhost:5433/userdb?sslmode=disable"),
		natsURL:      env("NATS_URL", "nats://localhost:4222"),
		otelEndpoint: env("OTEL_ENDPOINT", "localhost:4317"),
		webBaseURL:   env("WEB_BASE_URL", "http://localhost:3000"),
		accessTTL:    15 * time.Minute,
		refreshTTL:   7 * 24 * time.Hour,
	}
	secret, devFallback, err := resolveJWTSecret(cfg.jwtSecret, env("APP_ENV", "development"))
	if err != nil {
		// Fail CLOSED: a missing/weak secret in production means forgeable tokens.
		log.Error("refusing to start", "err", err)
		os.Exit(1)
	}
	if devFallback {
		log.Warn("JWT_SECRET not set — using an insecure dev default; DO NOT use in production")
	}
	cfg.jwtSecret = secret

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

	// Connect to NATS so registration can emit user.registered. Best-effort: if
	// NATS is down we log and continue (the publish itself is non-fatal too).
	nc, js, err := events.Connect(ctx, cfg.natsURL, events.StreamName, events.StreamSubjects())
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}
	defer nc.Close()
	log.Info("connected to nats jetstream")

	repo := store.NewPostgres(pool)
	// Same Postgres struct backs both the user repo and the refresh-token store;
	// verification and password-reset tokens live in separate types (distinct
	// Save/Get method sets).
	verifTokens := store.NewPostgresVerificationTokens(pool)
	resetTokens := store.NewPostgresPasswordResetTokens(pool)
	addresses := store.NewPostgresAddresses(pool)
	accessMgr := auth.NewJWTManager(cfg.jwtSecret, cfg.accessTTL, auth.TypeAccess)
	refreshMgr := auth.NewJWTManager(cfg.jwtSecret, cfg.refreshTTL, auth.TypeRefresh)
	userSrv := server.NewServer(repo, repo, verifTokens, resetTokens, addresses, accessMgr, refreshMgr, accessMgr, refreshMgr, events.NewNATSPublisher(js), cfg.webBaseURL, log)

	shutdownTracer, err := observability.InitTracer(ctx, "user", cfg.otelEndpoint)
	if err != nil {
		log.Warn("failed to init tracer, continuing without tracing", "err", err)
	} else {
		defer func() {
			if err := shutdownTracer(context.Background()); err != nil {
				log.Error("tracer shutdown", "err", err)
			}
		}()
		log.Info("opentelemetry tracing enabled", "endpoint", cfg.otelEndpoint)
	}

	// Interceptors run in chain order: logging (outer) wraps metrics wraps recovery
	// (inner) wraps the handler — so logging and metrics both record the final status
	// even when recovery has turned a panic into an Internal error. Metrics registers
	// against the default registry, which is exactly what /metrics (promhttp) serves.
	metrics := grpcmw.NewMetrics(prometheus.DefaultRegisterer, "user")
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			grpcmw.UnaryLogging(log),
			grpcmw.UnaryMetrics(metrics),
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

// devJWTSecret is the public fallback used ONLY outside production.
const devJWTSecret = "dev-insecure-secret-change-me"

// resolveJWTSecret decides the signing secret for the given environment. In
// production it requires a strong, non-default secret and returns an error
// otherwise (the caller fails closed). In any other environment a blank secret
// falls back to the dev default (devFallback=true so the caller can warn).
func resolveJWTSecret(secret, appEnv string) (resolved string, devFallback bool, err error) {
	if appEnv == "production" {
		if len(secret) < 32 || secret == devJWTSecret {
			return "", false, errors.New("JWT_SECRET must be a strong value (>= 32 chars) in production")
		}
		return secret, false, nil
	}
	if secret == "" {
		return devJWTSecret, true, nil
	}
	return secret, false, nil
}
