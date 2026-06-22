package server_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
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

// countingProvider wraps a Provider and counts Charge calls, so a test can prove
// an idempotent retry charges AT MOST ONCE.
type countingProvider struct {
	inner provider.Provider
	calls int32
}

func (c *countingProvider) Charge(ctx context.Context, amount int64, currency, ref string) (string, error) {
	atomic.AddInt32(&c.calls, 1)
	return c.inner.Charge(ctx, amount, currency, ref)
}

func newClient(t *testing.T, prov provider.Provider) paymentv1.PaymentServiceClient {
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

	srv := server.NewServer(pool, prov, slog.New(slog.NewTextHandler(io.Discard, nil)))
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

func req(amount int64, key string) *paymentv1.CreatePaymentRequest {
	return &paymentv1.CreatePaymentRequest{OrderId: uuid.NewString(), AmountCents: amount, Currency: "NGN", IdempotencyKey: key}
}

func TestCreatePayment_SucceedsAndDeclines(t *testing.T) {
	ctx := context.Background()
	c := newClient(t, provider.NewMock())

	ok, err := c.CreatePayment(ctx, req(2500, uuid.NewString()))
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if ok.GetStatus() != "succeeded" || ok.GetPaymentId() == "" {
		t.Errorf("ok payment = %+v, want succeeded", ok)
	}

	// amount % 100 == 13 -> declined.
	dec, err := c.CreatePayment(ctx, req(1313, uuid.NewString()))
	if err != nil {
		t.Fatalf("CreatePayment(declined): %v", err)
	}
	if dec.GetStatus() != "failed" {
		t.Errorf("declined payment status = %q, want failed", dec.GetStatus())
	}
}

// TestCreatePayment_Idempotent is the key test: the same idempotency_key, twice,
// returns the SAME payment and charges the provider exactly once.
func TestCreatePayment_Idempotent(t *testing.T) {
	ctx := context.Background()
	cp := &countingProvider{inner: provider.NewMock()}
	c := newClient(t, cp)

	key := uuid.NewString()
	r := req(2500, key)

	first, err := c.CreatePayment(ctx, r)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := c.CreatePayment(ctx, r)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}

	if first.GetPaymentId() != second.GetPaymentId() {
		t.Errorf("payment ids differ: %s vs %s — retry created a second payment", first.GetPaymentId(), second.GetPaymentId())
	}
	if n := atomic.LoadInt32(&cp.calls); n != 1 {
		t.Errorf("provider charged %d times, want exactly 1", n)
	}
}

func TestCreatePayment_Validation(t *testing.T) {
	ctx := context.Background()
	c := newClient(t, provider.NewMock())

	if _, err := c.CreatePayment(ctx, &paymentv1.CreatePaymentRequest{OrderId: "nope", AmountCents: 1, IdempotencyKey: "k"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("bad order_id: want InvalidArgument, got %v", err)
	}
	if _, err := c.CreatePayment(ctx, &paymentv1.CreatePaymentRequest{OrderId: uuid.NewString(), AmountCents: 1, IdempotencyKey: ""}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("empty key: want InvalidArgument, got %v", err)
	}
}

func TestGetPayment(t *testing.T) {
	ctx := context.Background()
	c := newClient(t, provider.NewMock())

	created, _ := c.CreatePayment(ctx, req(2500, uuid.NewString()))
	got, err := c.GetPayment(ctx, &paymentv1.GetPaymentRequest{PaymentId: created.GetPaymentId()})
	if err != nil {
		t.Fatalf("GetPayment: %v", err)
	}
	if got.GetPayment().GetStatus() != "succeeded" || got.GetPayment().GetProviderRef() == "" {
		t.Errorf("payment = %+v", got.GetPayment())
	}

	if _, err := c.GetPayment(ctx, &paymentv1.GetPaymentRequest{PaymentId: uuid.NewString()}); status.Code(err) != codes.NotFound {
		t.Errorf("missing payment: want NotFound, got %v", err)
	}
}
