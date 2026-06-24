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

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/menawar/ecommerce-platform/pkg/httputil"
	"github.com/menawar/ecommerce-platform/pkg/observability"
	cartv1 "github.com/menawar/ecommerce-platform/proto/cart/v1"
	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/gateway/internal/gateway"
)

func main() {
	log := observability.NewLogger("gateway")
	httpAddr := env("GATEWAY_HTTP_ADDR", ":8080")
	userAddr := env("USER_GRPC_ADDR", "localhost:50051")
	productAddr := env("PRODUCT_GRPC_ADDR", "localhost:50052")
	cartAddr := env("CART_GRPC_ADDR", "localhost:50053")
	orderAddr := env("ORDER_GRPC_ADDR", "localhost:50055")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log, httpAddr, userAddr, productAddr, cartAddr, orderAddr); err != nil {
		log.Error("server exited with error", "err", err)
		os.Exit(1)
	}
	log.Info("server stopped cleanly")
}

func run(ctx context.Context, log *slog.Logger, httpAddr, userAddr, productAddr, cartAddr, orderAddr string) error {
	// grpc.NewClient creates a lazily-connecting client: it does NOT dial here,
	// it connects on the first RPC and reconnects automatically. So the gateway
	// can start before the backing services are reachable. insecure creds =
	// plaintext, fine inside the trusted compose network; TLS terminates at this edge.
	userConn, err := grpc.NewClient(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create user client: %w", err)
	}
	defer func() { _ = userConn.Close() }()

	productConn, err := grpc.NewClient(productAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create product client: %w", err)
	}
	defer func() { _ = productConn.Close() }()

	cartConn, err := grpc.NewClient(cartAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create cart client: %w", err)
	}
	defer func() { _ = cartConn.Close() }()

	orderConn, err := grpc.NewClient(orderAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create order client: %w", err)
	}
	defer func() { _ = orderConn.Close() }()

	httpMetrics := httputil.NewHTTPMetrics(prometheus.DefaultRegisterer, "gateway")

	h := gateway.NewHandler(
		userv1.NewUserServiceClient(userConn),
		productv1.NewProductServiceClient(productConn),
		cartv1.NewCartServiceClient(cartConn),
		orderv1.NewOrderServiceClient(orderConn),
		httpMetrics,
		log,
	)
	httpServer := &http.Server{
		Addr:              httpAddr,
		Handler:           h.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Info("gateway listening", "addr", httpAddr, "user_grpc", userAddr, "product_grpc", productAddr, "cart_grpc", cartAddr, "order_grpc", orderAddr)
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
