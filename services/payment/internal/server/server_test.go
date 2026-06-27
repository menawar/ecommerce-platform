package server_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/menawar/ecommerce-platform/pkg/postgres"
	paymentv1 "github.com/menawar/ecommerce-platform/proto/payment/v1"
	"github.com/menawar/ecommerce-platform/services/payment/internal/provider"
	"github.com/menawar/ecommerce-platform/services/payment/internal/server"
)

// newClient spins up the PaymentService over an in-memory bufconn, backed by a
// real paymentdb (skips if unavailable) and the Mock async provider.
func newClient(t *testing.T) paymentv1.PaymentServiceClient {
	t.Helper()
	url := os.Getenv("PAYMENT_DB_URL")
	if url == "" {
		url = "postgres://ecommerce:ecommerce@localhost:5433/paymentdb?sslmode=disable"
	}
	pool, err := postgres.NewPool(context.Background(), url)
	if err != nil {
		t.Skipf("skipping integration test (paymentdb unavailable; run `make infra-up && make payment-migrate-up`): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "TRUNCATE payments"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	srv := server.NewServer(pool, slog.New(slog.NewTextHandler(io.Discard, nil))).
		WithAsync(provider.NameMock, provider.NewMock())
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer()
	paymentv1.RegisterPaymentServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return paymentv1.NewPaymentServiceClient(conn)
}

func initReq(amount int64, key string) *paymentv1.InitializePaymentRequest {
	return &paymentv1.InitializePaymentRequest{
		OrderId: uuid.NewString(), AmountCents: amount, Currency: "NGN",
		IdempotencyKey: key, Email: "buyer@example.com",
	}
}

// TestInitializePaymentOverGRPC checks the happy response shape end-to-end through
// the gRPC layer (idempotency itself is covered in server_async_test.go).
func TestInitializePaymentOverGRPC(t *testing.T) {
	c := newClient(t)
	resp, err := c.InitializePayment(context.Background(), initReq(2500, uuid.NewString()))
	if err != nil {
		t.Fatalf("InitializePayment: %v", err)
	}
	if resp.GetStatus() != "pending" || resp.GetAuthorizationUrl() == "" || resp.GetPaymentId() == "" {
		t.Errorf("want pending + authorization_url + id, got %+v", resp)
	}
}

func TestInitializePayment_Validation(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)

	if _, err := c.InitializePayment(ctx, &paymentv1.InitializePaymentRequest{OrderId: "nope", IdempotencyKey: "k", Email: "e@x.com"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("bad order_id: want InvalidArgument, got %v", err)
	}
	if _, err := c.InitializePayment(ctx, &paymentv1.InitializePaymentRequest{OrderId: uuid.NewString(), IdempotencyKey: "", Email: "e@x.com"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("empty key: want InvalidArgument, got %v", err)
	}
	if _, err := c.InitializePayment(ctx, &paymentv1.InitializePaymentRequest{OrderId: uuid.NewString(), IdempotencyKey: "k2", Email: ""}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("empty email: want InvalidArgument, got %v", err)
	}
}

func TestGetPayment(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)

	created, err := c.InitializePayment(ctx, initReq(2500, uuid.NewString()))
	if err != nil {
		t.Fatalf("InitializePayment: %v", err)
	}
	got, err := c.GetPayment(ctx, &paymentv1.GetPaymentRequest{PaymentId: created.GetPaymentId()})
	if err != nil {
		t.Fatalf("GetPayment: %v", err)
	}
	if got.GetPayment().GetStatus() != "pending" || got.GetPayment().GetProviderRef() == "" {
		t.Errorf("payment = %+v, want pending with a provider_ref", got.GetPayment())
	}

	if _, err := c.GetPayment(ctx, &paymentv1.GetPaymentRequest{PaymentId: uuid.NewString()}); status.Code(err) != codes.NotFound {
		t.Errorf("missing payment: want NotFound, got %v", err)
	}
}
