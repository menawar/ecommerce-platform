package server_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/menawar/ecommerce-platform/pkg/postgres"
	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
	"github.com/menawar/ecommerce-platform/services/order/internal/db"
	"github.com/menawar/ecommerce-platform/services/order/internal/saga"
	"github.com/menawar/ecommerce-platform/services/order/internal/server"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("ORDER_DB_URL")
	if url == "" {
		url = "postgres://ecommerce:ecommerce@localhost:5433/orderdb?sslmode=disable"
	}
	pool, err := postgres.NewPool(context.Background(), url)
	if err != nil {
		t.Skipf("skipping integration test (orderdb unavailable): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "TRUNCATE orders, order_items, outbox CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

// newClient wires a server over a pool. The saga has nil clients — fine, because
// these tests only exercise GetOrder/ListOrders and Cancel-of-a-confirmed-order,
// none of which call out to Cart/Product/Payment.
func newClient(t *testing.T, pool *pgxpool.Pool) orderv1.OrderServiceClient {
	t.Helper()
	sg := saga.New(pool, nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	srv := server.NewServer(pool, sg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer()
	orderv1.RegisterOrderServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return orderv1.NewOrderServiceClient(conn)
}

// seedOrder inserts a CONFIRMED order with one item directly, returning ids.
func seedOrder(t *testing.T, pool *pgxpool.Pool) (orderID, userID string) {
	t.Helper()
	ctx := context.Background()
	q := db.New(pool)
	oid := uuid.New()
	uid := uuid.New()
	key := uuid.NewString()
	if _, err := q.CreateOrder(ctx, db.CreateOrderParams{
		ID: pgUUID(oid), UserID: pgUUID(uid), Status: "CONFIRMED", TotalCents: 2500,
		Currency: "NGN", ReservationID: pgUUID(oid), IdempotencyKey: &key,
	}); err != nil {
		t.Fatalf("seed order: %v", err)
	}
	if err := q.CreateOrderItem(ctx, db.CreateOrderItemParams{
		OrderID: pgUUID(oid), ProductID: pgUUID(uuid.New()), Name: "Widget", PriceCents: 2500, Quantity: 1,
	}); err != nil {
		t.Fatalf("seed item: %v", err)
	}
	return oid.String(), uid.String()
}

func pgUUID(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }

func TestGetOrder(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	c := newClient(t, pool)
	orderID, _ := seedOrder(t, pool)

	got, err := c.GetOrder(ctx, &orderv1.GetOrderRequest{OrderId: orderID})
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	o := got.GetOrder()
	if o.GetStatus() != "CONFIRMED" || o.GetTotalCents() != 2500 || len(o.GetItems()) != 1 {
		t.Errorf("order = %+v", o)
	}
	if o.GetItems()[0].GetName() != "Widget" {
		t.Errorf("item name = %q", o.GetItems()[0].GetName())
	}

	if _, err := c.GetOrder(ctx, &orderv1.GetOrderRequest{OrderId: uuid.NewString()}); status.Code(err) != codes.NotFound {
		t.Errorf("missing order: want NotFound, got %v", err)
	}
}

func TestListOrders(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	c := newClient(t, pool)
	orderID, userID := seedOrder(t, pool)

	resp, err := c.ListOrders(ctx, &orderv1.ListOrdersRequest{UserId: userID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}
	if len(resp.GetOrders()) != 1 || resp.GetOrders()[0].GetId() != orderID {
		t.Errorf("orders = %+v", resp.GetOrders())
	}
}

func TestCancelOrder_PaidFails(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	c := newClient(t, pool)
	orderID, _ := seedOrder(t, pool) // CONFIRMED

	if _, err := c.CancelOrder(ctx, &orderv1.CancelOrderRequest{OrderId: orderID}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("cancel confirmed order: want FailedPrecondition, got %v", err)
	}
}
