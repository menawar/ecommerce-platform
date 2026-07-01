// Command paymentd runs the Payment service: payment.v1.PaymentService over a pgx
// pool, with the MockProvider. Same daemon skeleton as the others.
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

	"github.com/menawar/ecommerce-platform/pkg/events"
	"github.com/menawar/ecommerce-platform/pkg/grpcmw"
	"github.com/menawar/ecommerce-platform/pkg/observability"
	"github.com/menawar/ecommerce-platform/pkg/outbox"
	"github.com/menawar/ecommerce-platform/pkg/postgres"
	paymentv1 "github.com/menawar/ecommerce-platform/proto/payment/v1"
	"github.com/menawar/ecommerce-platform/services/payment/internal/outboxstore"
	"github.com/menawar/ecommerce-platform/services/payment/internal/provider"
	"github.com/menawar/ecommerce-platform/services/payment/internal/server"
)

type config struct {
	grpcAddr     string
	httpAddr     string
	dbURL        string
	natsURL      string
	otelEndpoint string

	// Async payment provider. paymentProvider selects mock|paystack. The Paystack
	// secret key authenticates API calls; webhookSecret verifies inbound webhook
	// signatures (Paystack signs with the secret key, so it defaults to it).
	paymentProvider string
	paystackSecret  string
	paystackBaseURL string
	webhookSecret   string
}

func main() {
	log := observability.NewLogger("payment")
	cfg := config{
		grpcAddr:     env("PAYMENT_GRPC_ADDR", ":50054"),
		httpAddr:     env("PAYMENT_HTTP_ADDR", ":2115"),
		dbURL:        env("PAYMENT_DB_URL", "postgres://ecommerce:ecommerce@localhost:5433/paymentdb?sslmode=disable"),
		natsURL:      env("NATS_URL", "nats://localhost:4222"),
		otelEndpoint: env("OTEL_ENDPOINT", "localhost:4317"),

		paymentProvider: env("PAYMENT_PROVIDER", provider.NameMock),
		paystackSecret:  os.Getenv("PAYSTACK_SECRET_KEY"),
		paystackBaseURL: os.Getenv("PAYSTACK_BASE_URL"),
		webhookSecret:   os.Getenv("PAYSTACK_WEBHOOK_SECRET"),
	}
	// Resolve the webhook signing secret. Paystack signs webhooks with the account
	// secret key, so default to that when running against Paystack; fall back to a
	// dev value only for the mock provider (whose simulator signs with the same).
	if cfg.webhookSecret == "" {
		if cfg.paymentProvider == provider.NamePaystack && cfg.paystackSecret != "" {
			cfg.webhookSecret = cfg.paystackSecret
		} else {
			cfg.webhookSecret = "dev-webhook-secret"
		}
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
		return fmt.Errorf("connect paymentdb: %w", err)
	}
	defer pool.Close()
	log.Info("connected to paymentdb")

	shutdownTracer, err := observability.InitTracer(ctx, "payment", cfg.otelEndpoint)
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

	// Connect to NATS JetStream and ensure the shared EVENTS stream exists. The
	// outbox poller drains payment.* events (webhook-driven, added next step) to NATS.
	nc, js, err := events.Connect(ctx, cfg.natsURL, events.StreamName, events.StreamSubjects())
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}
	defer nc.Close()
	log.Info("connected to nats jetstream")

	poller := outbox.NewPoller(outboxstore.New(pool), events.NewNATSPublisher(js), log, outbox.WithInterval(time.Second))

	reporter := observability.NewReporter(env("SENTRY_DSN", ""), "payment", env("ENVIRONMENT", "development"), log)
	defer reporter.Close()
	metrics := grpcmw.NewMetrics(prometheus.DefaultRegisterer, "payment")
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(grpcmw.UnaryLogging(log), grpcmw.UnaryMetrics(metrics), grpcmw.UnaryErrorReporting(reporter), grpcmw.UnaryRecovery(log)),
	)
	// The payment provider (mock|paystack) drives the async charge + webhook flow.
	asyncName, asyncProv := buildAsyncProvider(cfg, log)
	paymentSrv := server.NewServer(pool, log).WithAsync(asyncName, asyncProv)
	paymentv1.RegisterPaymentServiceServer(grpcServer, paymentSrv)
	reflection.Register(grpcServer)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// Paystack posts payment outcomes here; the handler verifies the HMAC signature
	// and finalizes the payment (emitting payment.succeeded/failed via the outbox).
	mux.HandleFunc("POST /webhooks/paystack", paymentSrv.PaystackWebhookHandler(cfg.webhookSecret))
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
	// The outbox poller runs as its own goroutine, alongside the servers.
	g.Go(func() error {
		log.Info("outbox poller started")
		return poller.Run(ctx)
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

// buildAsyncProvider selects the async (redirect+webhook) provider from config.
// Default is the deterministic Mock so the stack runs offline; "paystack" requires
// a secret key and falls back to Mock (with a warning) if it's missing.
func buildAsyncProvider(cfg config, log *slog.Logger) (string, provider.AsyncProvider) {
	if cfg.paymentProvider == provider.NamePaystack {
		if cfg.paystackSecret == "" {
			log.Warn("PAYMENT_PROVIDER=paystack but PAYSTACK_SECRET_KEY is empty; falling back to mock")
			return provider.NameMock, provider.NewMock()
		}
		var opts []provider.PaystackOption
		if cfg.paystackBaseURL != "" {
			opts = append(opts, provider.WithPaystackBaseURL(cfg.paystackBaseURL))
		}
		log.Info("using paystack payment provider")
		return provider.NamePaystack, provider.NewPaystack(cfg.paystackSecret, opts...)
	}
	return provider.NameMock, provider.NewMock()
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
