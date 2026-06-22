package saga_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/menawar/ecommerce-platform/pkg/postgres"
	cartv1 "github.com/menawar/ecommerce-platform/proto/cart/v1"
	paymentv1 "github.com/menawar/ecommerce-platform/proto/payment/v1"
	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	"github.com/menawar/ecommerce-platform/services/order/internal/db"
	"github.com/menawar/ecommerce-platform/services/order/internal/order"
	"github.com/menawar/ecommerce-platform/services/order/internal/saga"
)

// --- fakes for the three service clients ---

type fakeCart struct {
	items   []*cartv1.CartItem
	cleared bool
}

func (f *fakeCart) GetCart(context.Context, *cartv1.GetCartRequest, ...grpc.CallOption) (*cartv1.GetCartResponse, error) {
	return &cartv1.GetCartResponse{Cart: &cartv1.Cart{Items: f.items}}, nil
}
func (f *fakeCart) ClearCart(context.Context, *cartv1.ClearCartRequest, ...grpc.CallOption) (*cartv1.ClearCartResponse, error) {
	f.cleared = true
	return &cartv1.ClearCartResponse{}, nil
}
func (f *fakeCart) AddItem(context.Context, *cartv1.AddItemRequest, ...grpc.CallOption) (*cartv1.AddItemResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeCart) UpdateItem(context.Context, *cartv1.UpdateItemRequest, ...grpc.CallOption) (*cartv1.UpdateItemResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeCart) RemoveItem(context.Context, *cartv1.RemoveItemRequest, ...grpc.CallOption) (*cartv1.RemoveItemResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

type fakeProduct struct {
	products       map[string]*productv1.Product
	reserveSuccess bool
	reserved, committed, released bool
}

func (f *fakeProduct) GetProduct(_ context.Context, in *productv1.GetProductRequest, _ ...grpc.CallOption) (*productv1.GetProductResponse, error) {
	p, ok := f.products[in.GetProductId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "product not found")
	}
	return &productv1.GetProductResponse{Product: p}, nil
}
func (f *fakeProduct) ReserveStock(context.Context, *productv1.ReserveStockRequest, ...grpc.CallOption) (*productv1.ReserveStockResponse, error) {
	f.reserved = true
	return &productv1.ReserveStockResponse{Success: f.reserveSuccess}, nil
}
func (f *fakeProduct) CommitStock(context.Context, *productv1.CommitStockRequest, ...grpc.CallOption) (*productv1.CommitStockResponse, error) {
	f.committed = true
	return &productv1.CommitStockResponse{}, nil
}
func (f *fakeProduct) ReleaseStock(context.Context, *productv1.ReleaseStockRequest, ...grpc.CallOption) (*productv1.ReleaseStockResponse, error) {
	f.released = true
	return &productv1.ReleaseStockResponse{}, nil
}
func (f *fakeProduct) CreateProduct(context.Context, *productv1.CreateProductRequest, ...grpc.CallOption) (*productv1.CreateProductResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeProduct) ListProducts(context.Context, *productv1.ListProductsRequest, ...grpc.CallOption) (*productv1.ListProductsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

type fakePayment struct{ calls int }

// CreatePayment mirrors the real MockProvider rule: amount % 100 == 13 -> failed.
func (f *fakePayment) CreatePayment(_ context.Context, in *paymentv1.CreatePaymentRequest, _ ...grpc.CallOption) (*paymentv1.CreatePaymentResponse, error) {
	f.calls++
	st := "succeeded"
	if in.GetAmountCents()%100 == 13 {
		st = "failed"
	}
	return &paymentv1.CreatePaymentResponse{PaymentId: uuid.NewString(), Status: st}, nil
}
func (f *fakePayment) GetPayment(context.Context, *paymentv1.GetPaymentRequest, ...grpc.CallOption) (*paymentv1.GetPaymentResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// --- harness ---

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("ORDER_DB_URL")
	if url == "" {
		url = "postgres://ecommerce:ecommerce@localhost:5433/orderdb?sslmode=disable"
	}
	pool, err := postgres.NewPool(context.Background(), url)
	if err != nil {
		t.Skipf("skipping integration test (orderdb unavailable; run `make infra-up && make order-migrate-up`): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "TRUNCATE orders, order_items, outbox CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

func cartItem(price int64) (string, *cartv1.CartItem, *productv1.Product) {
	pid := uuid.NewString()
	return pid, &cartv1.CartItem{ProductId: pid, Quantity: 1}, &productv1.Product{Id: pid, Name: "Widget", PriceCents: price, Currency: "NGN", Available: 100}
}

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func outboxTopics(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := db.New(pool).ListUnpublishedOutbox(context.Background(), 100)
	if err != nil {
		t.Fatalf("list outbox: %v", err)
	}
	var topics []string
	for _, r := range rows {
		topics = append(topics, r.Topic)
	}
	return topics
}

func orderStatus(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	oid, _ := uuid.Parse(id)
	o, err := db.New(pool).GetOrder(context.Background(), pgtype.UUID{Bytes: oid, Valid: true})
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	return o.Status
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// TestSaga_HappyPath: stock available + payment succeeds -> CONFIRMED, stock
// committed, cart cleared, order.paid + order.confirmed in the outbox.
func TestSaga_HappyPath(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	pid, ci, prod := cartItem(2500) // total 2500, not %100==13
	cart := &fakeCart{items: []*cartv1.CartItem{ci}}
	product := &fakeProduct{products: map[string]*productv1.Product{pid: prod}, reserveSuccess: true}
	payment := &fakePayment{}
	s := saga.New(pool, cart, product, payment, discard())

	res, err := s.PlaceOrder(ctx, uuid.NewString(), uuid.NewString())
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if res.Status != order.StatusConfirmed {
		t.Fatalf("status = %s, want CONFIRMED", res.Status)
	}
	if !product.reserved || !product.committed || !cart.cleared {
		t.Errorf("expected reserve+commit+clear; reserved=%v committed=%v cleared=%v", product.reserved, product.committed, cart.cleared)
	}
	if product.released {
		t.Error("ReleaseStock should NOT be called on the happy path")
	}
	if orderStatus(t, pool, res.OrderID) != "CONFIRMED" {
		t.Error("db order not CONFIRMED")
	}
	topics := outboxTopics(t, pool)
	if !contains(topics, "order.paid") || !contains(topics, "order.confirmed") {
		t.Errorf("outbox topics = %v, want order.paid + order.confirmed", topics)
	}
}

// TestSaga_PaymentDeclined is the MANDATORY compensation test: a declined charge
// must RELEASE the reservation and end CANCELLED — no stock leak, no commit.
func TestSaga_PaymentDeclined(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	pid, ci, prod := cartItem(1313) // total 1313 -> %100==13 -> declined
	cart := &fakeCart{items: []*cartv1.CartItem{ci}}
	product := &fakeProduct{products: map[string]*productv1.Product{pid: prod}, reserveSuccess: true}
	s := saga.New(pool, cart, product, &fakePayment{}, discard())

	res, err := s.PlaceOrder(ctx, uuid.NewString(), uuid.NewString())
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if res.Status != order.StatusCancelled {
		t.Fatalf("status = %s, want CANCELLED", res.Status)
	}
	if !product.reserved || !product.released {
		t.Errorf("expected reserve THEN release; reserved=%v released=%v", product.reserved, product.released)
	}
	if product.committed {
		t.Error("CommitStock must NOT be called when payment fails")
	}
	if cart.cleared {
		t.Error("cart must NOT be cleared on a cancelled order")
	}
	if orderStatus(t, pool, res.OrderID) != "CANCELLED" {
		t.Error("db order not CANCELLED")
	}
	topics := outboxTopics(t, pool)
	if !contains(topics, "order.cancelled") || contains(topics, "order.paid") {
		t.Errorf("outbox topics = %v, want order.cancelled (no order.paid)", topics)
	}
}

// TestSaga_InsufficientStock: reservation fails -> CANCELLED, payment never called.
func TestSaga_InsufficientStock(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	pid, ci, prod := cartItem(2500)
	cart := &fakeCart{items: []*cartv1.CartItem{ci}}
	product := &fakeProduct{products: map[string]*productv1.Product{pid: prod}, reserveSuccess: false}
	payment := &fakePayment{}
	s := saga.New(pool, cart, product, payment, discard())

	res, err := s.PlaceOrder(ctx, uuid.NewString(), uuid.NewString())
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if res.Status != order.StatusCancelled {
		t.Errorf("status = %s, want CANCELLED", res.Status)
	}
	if payment.calls != 0 {
		t.Error("payment should not be attempted when stock can't be reserved")
	}
	if product.released {
		t.Error("nothing was reserved, so ReleaseStock should not be called")
	}
}

// TestSaga_Idempotent: same idempotency_key twice -> same order, processed once.
func TestSaga_Idempotent(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	pid, ci, prod := cartItem(2500)
	cart := &fakeCart{items: []*cartv1.CartItem{ci}}
	product := &fakeProduct{products: map[string]*productv1.Product{pid: prod}, reserveSuccess: true}
	payment := &fakePayment{}
	s := saga.New(pool, cart, product, payment, discard())

	key := uuid.NewString()
	user := uuid.NewString()
	first, err := s.PlaceOrder(ctx, user, key)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := s.PlaceOrder(ctx, user, key)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if first.OrderID != second.OrderID {
		t.Errorf("replay made a new order: %s vs %s", first.OrderID, second.OrderID)
	}
	if payment.calls != 1 {
		t.Errorf("payment called %d times, want 1 (replay must not re-charge)", payment.calls)
	}
}

func TestSaga_EmptyCart(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	s := saga.New(pool, &fakeCart{}, &fakeProduct{products: map[string]*productv1.Product{}}, &fakePayment{}, discard())

	if _, err := s.PlaceOrder(ctx, uuid.NewString(), uuid.NewString()); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("empty cart: want FailedPrecondition, got %v", err)
	}
}
