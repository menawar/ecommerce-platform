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

	"github.com/menawar/ecommerce-platform/pkg/events"
	"github.com/menawar/ecommerce-platform/pkg/postgres"
	cartv1 "github.com/menawar/ecommerce-platform/proto/cart/v1"
	paymentv1 "github.com/menawar/ecommerce-platform/proto/payment/v1"
	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
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
	products                      map[string]*productv1.Product
	reserveSuccess                bool
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
func (f *fakeProduct) UpdateProduct(context.Context, *productv1.UpdateProductRequest, ...grpc.CallOption) (*productv1.UpdateProductResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeProduct) DeleteProduct(context.Context, *productv1.DeleteProductRequest, ...grpc.CallOption) (*productv1.DeleteProductResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeProduct) ListProducts(context.Context, *productv1.ListProductsRequest, ...grpc.CallOption) (*productv1.ListProductsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

type fakePayment struct {
	initCalls   int
	refundCalls int
	refundErr   error // when set, RefundPayment fails (models a PSP refund rejection)
}

// InitializePayment models the async PSP: it never decides the outcome — it just
// starts the charge and returns 'pending' + a hosted-checkout URL. The success/
// decline is delivered later via a payment.* event (see resume()).
func (f *fakePayment) InitializePayment(_ context.Context, in *paymentv1.InitializePaymentRequest, _ ...grpc.CallOption) (*paymentv1.InitializePaymentResponse, error) {
	f.initCalls++
	return &paymentv1.InitializePaymentResponse{
		PaymentId:        uuid.NewString(),
		Status:           "pending",
		AuthorizationUrl: "https://pay.test/" + in.GetOrderId(),
	}, nil
}
func (f *fakePayment) GetPayment(context.Context, *paymentv1.GetPaymentRequest, ...grpc.CallOption) (*paymentv1.GetPaymentResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakePayment) RefundPayment(_ context.Context, _ *paymentv1.RefundPaymentRequest, _ ...grpc.CallOption) (*paymentv1.RefundPaymentResponse, error) {
	f.refundCalls++
	if f.refundErr != nil {
		return nil, f.refundErr
	}
	return &paymentv1.RefundPaymentResponse{Status: "refunded"}, nil
}

// fakeUser resolves a customer's email (for payment) and address (for the order
// snapshot). addrErr, when set, makes GetAddress fail — e.g. a not-owned address.
type fakeUser struct{ addrErr error }

func (f fakeUser) GetAddress(_ context.Context, in *userv1.GetAddressRequest, _ ...grpc.CallOption) (*userv1.GetAddressResponse, error) {
	if f.addrErr != nil {
		return nil, f.addrErr
	}
	return &userv1.GetAddressResponse{Address: &userv1.Address{
		Id: in.GetAddressId(), UserId: in.GetUserId(), Recipient: "Ada Lovelace", Phone: "08030000000",
		Line1: "1 Rayfield Rd", City: "Jos", State: "Plateau", Country: "NG",
	}}, nil
}

func (fakeUser) GetUser(_ context.Context, in *userv1.GetUserRequest, _ ...grpc.CallOption) (*userv1.GetUserResponse, error) {
	return &userv1.GetUserResponse{UserId: in.GetUserId(), Email: "buyer@example.com"}, nil
}
func (fakeUser) Register(context.Context, *userv1.RegisterRequest, ...grpc.CallOption) (*userv1.RegisterResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) Login(context.Context, *userv1.LoginRequest, ...grpc.CallOption) (*userv1.LoginResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) ValidateToken(context.Context, *userv1.ValidateTokenRequest, ...grpc.CallOption) (*userv1.ValidateTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) RefreshToken(context.Context, *userv1.RefreshTokenRequest, ...grpc.CallOption) (*userv1.RefreshTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) Logout(context.Context, *userv1.LogoutRequest, ...grpc.CallOption) (*userv1.LogoutResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) VerifyEmail(context.Context, *userv1.VerifyEmailRequest, ...grpc.CallOption) (*userv1.VerifyEmailResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) ResendVerification(context.Context, *userv1.ResendVerificationRequest, ...grpc.CallOption) (*userv1.ResendVerificationResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) RequestPasswordReset(context.Context, *userv1.RequestPasswordResetRequest, ...grpc.CallOption) (*userv1.RequestPasswordResetResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) ResetPassword(context.Context, *userv1.ResetPasswordRequest, ...grpc.CallOption) (*userv1.ResetPasswordResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) CreateAddress(context.Context, *userv1.CreateAddressRequest, ...grpc.CallOption) (*userv1.CreateAddressResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) ListAddresses(context.Context, *userv1.ListAddressesRequest, ...grpc.CallOption) (*userv1.ListAddressesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) UpdateAddress(context.Context, *userv1.UpdateAddressRequest, ...grpc.CallOption) (*userv1.UpdateAddressResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) DeleteAddress(context.Context, *userv1.DeleteAddressRequest, ...grpc.CallOption) (*userv1.DeleteAddressResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
func (fakeUser) SetDefaultAddress(context.Context, *userv1.SetDefaultAddressRequest, ...grpc.CallOption) (*userv1.SetDefaultAddressResponse, error) {
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

// seedShipping inserts an active shipping method and returns its id, for the saga
// to resolve at checkout. Returns the id and its price so tests can assert totals.
func seedShipping(t *testing.T, pool *pgxpool.Pool, priceCents int64) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		"INSERT INTO shipping_methods (name, price_cents, active) VALUES ('Standard', $1, true) RETURNING id::text",
		priceCents).Scan(&id)
	if err != nil {
		t.Fatalf("seed shipping method: %v", err)
	}
	return id
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

// resume delivers a payment outcome through the saga's real consumer entrypoint
// (HandlePaymentEvent), exercising topic dispatch + envelope decoding, just as the
// NATS consumer does in production.
func resume(t *testing.T, s *saga.Saga, orderID string, paid bool) {
	t.Helper()
	topic := "payment.succeeded"
	if !paid {
		topic = "payment.failed"
	}
	env, err := events.New(topic, map[string]string{"order_id": orderID})
	if err != nil {
		t.Fatalf("build event: %v", err)
	}
	if err := s.HandlePaymentEvent(context.Background(), env); err != nil {
		t.Fatalf("HandlePaymentEvent(%s): %v", topic, err)
	}
}

// TestSaga_HappyPath: PlaceOrder reserves stock and pauses at PAYMENT_PENDING with
// an authorization URL; the payment.succeeded event then drives commit + clear +
// CONFIRMED, emitting order.paid + order.confirmed.
func TestSaga_HappyPath(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	pid, ci, prod := cartItem(2500)
	cart := &fakeCart{items: []*cartv1.CartItem{ci}}
	product := &fakeProduct{products: map[string]*productv1.Product{pid: prod}, reserveSuccess: true}
	payment := &fakePayment{}
	s := saga.New(pool, cart, product, payment, fakeUser{}, discard())

	// Phase 1: start. The saga pauses awaiting the customer's payment.
	res, err := s.PlaceOrder(ctx, uuid.NewString(), uuid.NewString(), uuid.NewString(), seedShipping(t, pool, 150000))
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if res.Status != order.StatusPaymentPending {
		t.Fatalf("status = %s, want PAYMENT_PENDING", res.Status)
	}
	if res.AuthorizationURL == "" {
		t.Error("expected an authorization_url to redirect the customer to")
	}
	if !product.reserved || product.committed || cart.cleared {
		t.Errorf("after start want reserve-only; reserved=%v committed=%v cleared=%v", product.reserved, product.committed, cart.cleared)
	}

	// Phase 2: payment succeeds -> resume to CONFIRMED.
	resume(t, s, res.OrderID, true)
	if orderStatus(t, pool, res.OrderID) != "CONFIRMED" {
		t.Error("db order not CONFIRMED after payment.succeeded")
	}
	if !product.committed || !cart.cleared {
		t.Errorf("after resume want commit+clear; committed=%v cleared=%v", product.committed, cart.cleared)
	}
	if product.released {
		t.Error("ReleaseStock should NOT be called on the happy path")
	}
	topics := outboxTopics(t, pool)
	if !contains(topics, "order.paid") || !contains(topics, "order.confirmed") {
		t.Errorf("outbox topics = %v, want order.paid + order.confirmed", topics)
	}
}

// TestSaga_PaymentDeclined is the MANDATORY compensation test, now async: the order
// reserves stock and waits, then a payment.failed event must RELEASE the
// reservation and end CANCELLED — no stock leak, no commit.
func TestSaga_PaymentDeclined(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	pid, ci, prod := cartItem(1313)
	cart := &fakeCart{items: []*cartv1.CartItem{ci}}
	product := &fakeProduct{products: map[string]*productv1.Product{pid: prod}, reserveSuccess: true}
	s := saga.New(pool, cart, product, &fakePayment{}, fakeUser{}, discard())

	res, err := s.PlaceOrder(ctx, uuid.NewString(), uuid.NewString(), uuid.NewString(), seedShipping(t, pool, 150000))
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if res.Status != order.StatusPaymentPending {
		t.Fatalf("status = %s, want PAYMENT_PENDING", res.Status)
	}

	// Payment declines -> resume must compensate.
	resume(t, s, res.OrderID, false)
	if orderStatus(t, pool, res.OrderID) != "CANCELLED" {
		t.Error("db order not CANCELLED after payment.failed")
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
	topics := outboxTopics(t, pool)
	if !contains(topics, "order.cancelled") || contains(topics, "order.paid") {
		t.Errorf("outbox topics = %v, want order.cancelled (no order.paid)", topics)
	}
}

// TestSaga_ResumeIdempotent: a redelivered payment.succeeded (at-least-once) must
// not double-confirm or re-clear — the second resume is a no-op.
func TestSaga_ResumeIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	pid, ci, prod := cartItem(2500)
	cart := &fakeCart{items: []*cartv1.CartItem{ci}}
	product := &fakeProduct{products: map[string]*productv1.Product{pid: prod}, reserveSuccess: true}
	s := saga.New(pool, cart, product, &fakePayment{}, fakeUser{}, discard())

	res, err := s.PlaceOrder(ctx, uuid.NewString(), uuid.NewString(), uuid.NewString(), seedShipping(t, pool, 150000))
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	resume(t, s, res.OrderID, true)
	resume(t, s, res.OrderID, true) // duplicate delivery

	if orderStatus(t, pool, res.OrderID) != "CONFIRMED" {
		t.Error("order not CONFIRMED")
	}
	// Exactly one order.confirmed despite two deliveries.
	var confirmed int
	for _, tp := range outboxTopics(t, pool) {
		if tp == "order.confirmed" {
			confirmed++
		}
	}
	if confirmed != 1 {
		t.Errorf("order.confirmed emitted %d times, want 1", confirmed)
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
	s := saga.New(pool, cart, product, payment, fakeUser{}, discard())

	res, err := s.PlaceOrder(ctx, uuid.NewString(), uuid.NewString(), uuid.NewString(), seedShipping(t, pool, 150000))
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if res.Status != order.StatusCancelled {
		t.Errorf("status = %s, want CANCELLED", res.Status)
	}
	if payment.initCalls != 0 {
		t.Error("payment should not be initialized when stock can't be reserved")
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
	s := saga.New(pool, cart, product, payment, fakeUser{}, discard())

	key := uuid.NewString()
	user := uuid.NewString()
	addr := uuid.NewString()
	ship := seedShipping(t, pool, 150000)
	first, err := s.PlaceOrder(ctx, user, key, addr, ship)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := s.PlaceOrder(ctx, user, key, addr, ship)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if first.OrderID != second.OrderID {
		t.Errorf("replay made a new order: %s vs %s", first.OrderID, second.OrderID)
	}
	if payment.initCalls != 1 {
		t.Errorf("payment initialized %d times, want 1 (replay must not re-initialize)", payment.initCalls)
	}
}

// TestSaga_TotalIncludesShippingAndSnapshot: total = subtotal + shipping, and the
// chosen method name + address are snapshotted onto the order.
func TestSaga_TotalIncludesShippingAndSnapshot(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	pid, ci, prod := cartItem(2500)
	cart := &fakeCart{items: []*cartv1.CartItem{ci}}
	product := &fakeProduct{products: map[string]*productv1.Product{pid: prod}, reserveSuccess: true}
	s := saga.New(pool, cart, product, &fakePayment{}, fakeUser{}, discard())

	ship := seedShipping(t, pool, 150000)
	res, err := s.PlaceOrder(ctx, uuid.NewString(), uuid.NewString(), uuid.NewString(), ship)
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	var total, shipping int64
	var methodName, recipient, city string
	if err := pool.QueryRow(ctx,
		"SELECT total_cents, shipping_cents, shipping_method_name, ship_recipient, ship_city FROM orders WHERE id=$1",
		res.OrderID).Scan(&total, &shipping, &methodName, &recipient, &city); err != nil {
		t.Fatalf("read order: %v", err)
	}
	if total != 2500+150000 {
		t.Errorf("total_cents = %d, want %d (subtotal+shipping)", total, 2500+150000)
	}
	if shipping != 150000 || methodName != "Standard" {
		t.Errorf("shipping snapshot = %d/%q, want 150000/Standard", shipping, methodName)
	}
	if recipient != "Ada Lovelace" || city != "Jos" {
		t.Errorf("address snapshot = %q/%q, want Ada Lovelace/Jos", recipient, city)
	}
}

func TestSaga_BadShippingOrAddressRejected(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	pid, ci, prod := cartItem(2500)
	mkSaga := func(usr fakeUser) *saga.Saga {
		cart := &fakeCart{items: []*cartv1.CartItem{ci}}
		product := &fakeProduct{products: map[string]*productv1.Product{pid: prod}, reserveSuccess: true}
		return saga.New(pool, cart, product, &fakePayment{}, usr, discard())
	}

	t.Run("inactive shipping method", func(t *testing.T) {
		var ship string
		_ = pool.QueryRow(ctx, "INSERT INTO shipping_methods (name, price_cents, active) VALUES ('Off', 1000, false) RETURNING id::text").Scan(&ship)
		_, err := mkSaga(fakeUser{}).PlaceOrder(ctx, uuid.NewString(), uuid.NewString(), uuid.NewString(), ship)
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("want FailedPrecondition, got %v", err)
		}
	})
	t.Run("unknown shipping method", func(t *testing.T) {
		_, err := mkSaga(fakeUser{}).PlaceOrder(ctx, uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString())
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("want FailedPrecondition, got %v", err)
		}
	})
	t.Run("unknown/not-owned address", func(t *testing.T) {
		ship := seedShipping(t, pool, 150000)
		usr := fakeUser{addrErr: status.Error(codes.NotFound, "address not found")}
		_, err := mkSaga(usr).PlaceOrder(ctx, uuid.NewString(), uuid.NewString(), uuid.NewString(), ship)
		if status.Code(err) != codes.FailedPrecondition {
			t.Errorf("want FailedPrecondition, got %v", err)
		}
	})
}

// confirmedOrder drives a fresh order all the way to CONFIRMED and returns its id.
func confirmedOrder(t *testing.T, s *saga.Saga, pool *pgxpool.Pool) string {
	t.Helper()
	res, err := s.PlaceOrder(context.Background(), uuid.NewString(), uuid.NewString(), uuid.NewString(), seedShipping(t, pool, 150000))
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	resume(t, s, res.OrderID, true)
	if orderStatus(t, pool, res.OrderID) != "CONFIRMED" {
		t.Fatalf("setup: order not CONFIRMED")
	}
	return res.OrderID
}

func fulfillmentSaga(t *testing.T, pool *pgxpool.Pool) *saga.Saga {
	t.Helper()
	pid, ci, prod := cartItem(2500)
	cart := &fakeCart{items: []*cartv1.CartItem{ci}}
	product := &fakeProduct{products: map[string]*productv1.Product{pid: prod}, reserveSuccess: true}
	return saga.New(pool, cart, product, &fakePayment{}, fakeUser{}, discard())
}

// TestSaga_ShipThenDeliver: CONFIRMED -> SHIPPED (tracking + order.shipped) ->
// DELIVERED (order.delivered).
func TestSaga_ShipThenDeliver(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	s := fulfillmentSaga(t, pool)
	oid := confirmedOrder(t, s, pool)

	shipped, err := s.MarkShipped(ctx, oid, "TRACK-123")
	if err != nil {
		t.Fatalf("MarkShipped: %v", err)
	}
	if shipped.Status != "SHIPPED" || shipped.TrackingNumber != "TRACK-123" || !shipped.ShippedAt.Valid {
		t.Errorf("after ship: %+v", shipped)
	}
	if !contains(outboxTopics(t, pool), "order.shipped") {
		t.Error("expected order.shipped event")
	}

	delivered, err := s.MarkDelivered(ctx, oid)
	if err != nil {
		t.Fatalf("MarkDelivered: %v", err)
	}
	if delivered.Status != "DELIVERED" || !delivered.DeliveredAt.Valid {
		t.Errorf("after deliver: %+v", delivered)
	}
	if !contains(outboxTopics(t, pool), "order.delivered") {
		t.Error("expected order.delivered event")
	}
}

func TestSaga_IllegalFulfillmentTransitions(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	s := fulfillmentSaga(t, pool)

	t.Run("ship a not-yet-confirmed order", func(t *testing.T) {
		// PlaceOrder pauses at PAYMENT_PENDING — not shippable.
		res, err := s.PlaceOrder(ctx, uuid.NewString(), uuid.NewString(), uuid.NewString(), seedShipping(t, pool, 150000))
		if err != nil {
			t.Fatalf("PlaceOrder: %v", err)
		}
		if _, err := s.MarkShipped(ctx, res.OrderID, ""); status.Code(err) != codes.FailedPrecondition {
			t.Errorf("ship PAYMENT_PENDING: want FailedPrecondition, got %v", err)
		}
	})
	t.Run("deliver a not-yet-shipped order", func(t *testing.T) {
		oid := confirmedOrder(t, s, pool)
		if _, err := s.MarkDelivered(ctx, oid); status.Code(err) != codes.FailedPrecondition {
			t.Errorf("deliver CONFIRMED: want FailedPrecondition, got %v", err)
		}
	})
	t.Run("ship an unknown order", func(t *testing.T) {
		if _, err := s.MarkShipped(ctx, uuid.NewString(), ""); status.Code(err) != codes.NotFound {
			t.Errorf("ship unknown: want NotFound, got %v", err)
		}
	})
}

// TestSaga_DuplicatePaymentAfterShipped: a redelivered payment.succeeded on an
// already-SHIPPED order must be a no-op (the broadened post-payment guard) — it
// must NOT drag the order back to CONFIRMED.
func TestSaga_DuplicatePaymentAfterShipped(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	s := fulfillmentSaga(t, pool)
	oid := confirmedOrder(t, s, pool)
	if _, err := s.MarkShipped(ctx, oid, "TRACK-1"); err != nil {
		t.Fatalf("MarkShipped: %v", err)
	}

	resume(t, s, oid, true) // duplicate payment.succeeded
	if got := orderStatus(t, pool, oid); got != "SHIPPED" {
		t.Errorf("status after duplicate payment = %s, want SHIPPED (unchanged)", got)
	}
}

func refundSaga(t *testing.T, pool *pgxpool.Pool) (*saga.Saga, *fakePayment) {
	t.Helper()
	pid, ci, prod := cartItem(2500)
	cart := &fakeCart{items: []*cartv1.CartItem{ci}}
	product := &fakeProduct{products: map[string]*productv1.Product{pid: prod}, reserveSuccess: true}
	pay := &fakePayment{}
	return saga.New(pool, cart, product, pay, fakeUser{}, discard()), pay
}

// TestSaga_RefundOrder: a CONFIRMED order refunds — payment reversed, order
// REFUNDED, order.refunded emitted; a second refund is an idempotent no-op.
func TestSaga_RefundOrder(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	s, pay := refundSaga(t, pool)
	oid := confirmedOrder(t, s, pool)

	o, err := s.RefundOrder(ctx, oid)
	if err != nil {
		t.Fatalf("RefundOrder: %v", err)
	}
	if o.Status != "REFUNDED" {
		t.Errorf("status = %s, want REFUNDED", o.Status)
	}
	if pay.refundCalls != 1 {
		t.Errorf("payment refunds = %d, want 1", pay.refundCalls)
	}
	if !contains(outboxTopics(t, pool), "order.refunded") {
		t.Error("expected order.refunded event")
	}

	// Idempotent: refunding again is a no-op and does not re-call the PSP.
	if _, err := s.RefundOrder(ctx, oid); err != nil {
		t.Fatalf("second RefundOrder: %v", err)
	}
	if pay.refundCalls != 1 {
		t.Errorf("payment refunds after replay = %d, want 1", pay.refundCalls)
	}
}

func TestSaga_RefundFromDelivered(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	s, _ := refundSaga(t, pool)
	oid := confirmedOrder(t, s, pool)
	if _, err := s.MarkShipped(ctx, oid, "T1"); err != nil {
		t.Fatalf("MarkShipped: %v", err)
	}
	if _, err := s.MarkDelivered(ctx, oid); err != nil {
		t.Fatalf("MarkDelivered: %v", err)
	}
	o, err := s.RefundOrder(ctx, oid)
	if err != nil || o.Status != "REFUNDED" {
		t.Errorf("refund from DELIVERED: status=%s err=%v", o.Status, err)
	}
}

func TestSaga_RefundGuards(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)

	t.Run("non-refundable state (payment pending)", func(t *testing.T) {
		s, pay := refundSaga(t, pool)
		res, err := s.PlaceOrder(ctx, uuid.NewString(), uuid.NewString(), uuid.NewString(), seedShipping(t, pool, 150000))
		if err != nil {
			t.Fatalf("PlaceOrder: %v", err)
		}
		if _, err := s.RefundOrder(ctx, res.OrderID); status.Code(err) != codes.FailedPrecondition {
			t.Errorf("refund PAYMENT_PENDING: want FailedPrecondition, got %v", err)
		}
		if pay.refundCalls != 0 {
			t.Error("PSP must not be called for a non-refundable order")
		}
	})

	t.Run("unknown order", func(t *testing.T) {
		s, _ := refundSaga(t, pool)
		if _, err := s.RefundOrder(ctx, uuid.NewString()); status.Code(err) != codes.NotFound {
			t.Errorf("refund unknown: want NotFound, got %v", err)
		}
	})

	t.Run("PSP refund failure leaves the order unchanged", func(t *testing.T) {
		s, pay := refundSaga(t, pool)
		pay.refundErr = status.Error(codes.Internal, "psp down")
		oid := confirmedOrder(t, s, pool)
		if _, err := s.RefundOrder(ctx, oid); err == nil {
			t.Fatal("expected an error when the PSP refund fails")
		}
		if got := orderStatus(t, pool, oid); got != "CONFIRMED" {
			t.Errorf("status after failed refund = %s, want CONFIRMED (unchanged)", got)
		}
	})
}

func TestSaga_EmptyCart(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	s := saga.New(pool, &fakeCart{}, &fakeProduct{products: map[string]*productv1.Product{}}, &fakePayment{}, fakeUser{}, discard())

	if _, err := s.PlaceOrder(ctx, uuid.NewString(), uuid.NewString(), uuid.NewString(), seedShipping(t, pool, 150000)); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("empty cart: want FailedPrecondition, got %v", err)
	}
}
