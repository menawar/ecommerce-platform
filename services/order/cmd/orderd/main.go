// Command orderd runs the Order service: the saga orchestrator's gRPC server, an
// HTTP server for /metrics and /healthz, AND the transactional-outbox poller — all
// as goroutines under one errgroup. It dials the Cart, Product, and Payment
// services (the saga's collaborators).
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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/menawar/ecommerce-platform/pkg/events"
	"github.com/menawar/ecommerce-platform/pkg/grpcmw"
	"github.com/menawar/ecommerce-platform/pkg/observability"
	"github.com/menawar/ecommerce-platform/pkg/outbox"
	"github.com/menawar/ecommerce-platform/pkg/postgres"
	cartv1 "github.com/menawar/ecommerce-platform/proto/cart/v1"
	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
	paymentv1 "github.com/menawar/ecommerce-platform/proto/payment/v1"
	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/order/internal/outboxstore"
	"github.com/menawar/ecommerce-platform/services/order/internal/saga"
	"github.com/menawar/ecommerce-platform/services/order/internal/server"
)

type config struct {
	grpcAddr     string
	httpAddr     string
	dbURL        string
	cartAddr     string
	productAddr  string
	paymentAddr  string
	userAddr     string
	natsURL      string
	otelEndpoint string
}

func main() {
	log := observability.NewLogger("order")
	cfg := config{
		grpcAddr:     env("ORDER_GRPC_ADDR", ":50055"),
		httpAddr:     env("ORDER_HTTP_ADDR", ":2116"),
		dbURL:        env("ORDER_DB_URL", "postgres://ecommerce:ecommerce@localhost:5433/orderdb?sslmode=disable"),
		cartAddr:     env("CART_GRPC_ADDR", "localhost:50053"),
		productAddr:  env("PRODUCT_GRPC_ADDR", "localhost:50052"),
		paymentAddr:  env("PAYMENT_GRPC_ADDR", "localhost:50054"),
		userAddr:     env("USER_GRPC_ADDR", "localhost:50051"),
		natsURL:      env("NATS_URL", "nats://localhost:4222"),
		otelEndpoint: env("OTEL_ENDPOINT", "localhost:4317"),
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
		return fmt.Errorf("connect orderdb: %w", err)
	}
	defer pool.Close()
	log.Info("connected to orderdb")

	// Lazy clients to the saga's collaborators.
	cartConn, err := dial(cfg.cartAddr)
	if err != nil {
		return fmt.Errorf("cart client: %w", err)
	}
	defer func() { _ = cartConn.Close() }()
	productConn, err := dial(cfg.productAddr)
	if err != nil {
		return fmt.Errorf("product client: %w", err)
	}
	defer func() { _ = productConn.Close() }()
	paymentConn, err := dial(cfg.paymentAddr)
	if err != nil {
		return fmt.Errorf("payment client: %w", err)
	}
	defer func() { _ = paymentConn.Close() }()
	userConn, err := dial(cfg.userAddr)
	if err != nil {
		return fmt.Errorf("user client: %w", err)
	}
	defer func() { _ = userConn.Close() }()

	sg := saga.New(pool,
		cartv1.NewCartServiceClient(cartConn),
		productv1.NewProductServiceClient(productConn),
		paymentv1.NewPaymentServiceClient(paymentConn),
		userv1.NewUserServiceClient(userConn),
		log,
	)

	shutdownTracer, err := observability.InitTracer(ctx, "order", cfg.otelEndpoint)
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

	metrics := grpcmw.NewMetrics(prometheus.DefaultRegisterer, "order")
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(grpcmw.UnaryLogging(log), grpcmw.UnaryMetrics(metrics), grpcmw.UnaryRecovery(log)),
	)
	orderv1.RegisterOrderServiceServer(grpcServer, server.NewServer(pool, sg, log))
	reflection.Register(grpcServer)

	// Connect to NATS JetStream and ensure the shared EVENTS stream exists. The
	// outbox poller now publishes to NATS instead of just logging — the Publisher
	// interface let us swap the implementation without touching the poller.
	nc, js, err := events.Connect(ctx, cfg.natsURL, events.StreamName, events.StreamSubjects())
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}
	defer nc.Close()
	log.Info("connected to nats jetstream")

	poller := outbox.NewPoller(outboxstore.New(pool), events.NewNATSPublisher(js), log, outbox.WithInterval(time.Second))

	// Resume consumer: the saga's "start" half leaves orders at PAYMENT_PENDING;
	// this durable consumer drives them to CONFIRMED/CANCELLED when the payment
	// service emits payment.succeeded/payment.failed. The handler is idempotent.
	resumeConsumer, err := events.Consume(ctx, js, events.StreamName, "order-saga", log, sg.HandleEvent)
	if err != nil {
		return fmt.Errorf("start payment-event consumer: %w", err)
	}
	defer resumeConsumer.Stop()

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

// dial creates a lazy gRPC client with OTel client instrumentation — so the
// order service's outgoing calls to product/payment/cart propagate the trace
// context, creating child spans under the saga's span.
func dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
