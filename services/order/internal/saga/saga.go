// Package saga orchestrates turning a cart into a paid order. It is the heart of
// the system: a sequence of cross-service calls (Cart, Product, Payment) with a
// state machine and COMPENSATING actions when a later step fails. Events are
// written to the outbox in the same DB transaction as the state change.
package saga

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cartv1 "github.com/menawar/ecommerce-platform/proto/cart/v1"
	paymentv1 "github.com/menawar/ecommerce-platform/proto/payment/v1"
	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	"github.com/menawar/ecommerce-platform/services/order/internal/db"
	"github.com/menawar/ecommerce-platform/services/order/internal/order"
)

// ErrEmptyCart is a normal outcome — you can't place an order with nothing in the
// cart. The server maps it to FailedPrecondition.
var ErrEmptyCart = errors.New("saga: cart is empty")

// Saga depends on the generated gRPC client INTERFACES, so tests inject fakes
// (cart contents, product prices, payment outcome) and exercise the full
// orchestration — including the compensation path — without real services.
type Saga struct {
	pool     *pgxpool.Pool
	q        *db.Queries
	carts    cartv1.CartServiceClient
	products productv1.ProductServiceClient
	payments paymentv1.PaymentServiceClient
	log      *slog.Logger
}

func New(
	pool *pgxpool.Pool,
	carts cartv1.CartServiceClient,
	products productv1.ProductServiceClient,
	payments paymentv1.PaymentServiceClient,
	log *slog.Logger,
) *Saga {
	return &Saga{pool: pool, q: db.New(pool), carts: carts, products: products, payments: payments, log: log}
}

// Result is what PlaceOrder reports back.
type Result struct {
	OrderID string
	Status  order.Status
}

// lineItem is a priced cart line, snapshotted from the Product service.
type lineItem struct {
	productID  string
	name       string
	priceCents int64
	quantity   int32
}

// orderEvent is the outbox payload. event_id makes each delivery uniquely
// identifiable so at-least-once consumers can dedupe (Phase 5).
type orderEvent struct {
	EventID    string `json:"event_id"`
	OrderID    string `json:"order_id"`
	UserID     string `json:"user_id"`
	TotalCents int64  `json:"total_cents"`
	Status     string `json:"status"`
}

// PlaceOrder runs the saga. Happy path:
//
//	PENDING -> reserve stock -> STOCK_RESERVED -> PAYMENT_PENDING -> charge
//	  -> commit stock + PAID (+order.paid) -> clear cart -> CONFIRMED (+order.confirmed)
//
// On insufficient stock or a declined charge it COMPENSATES (release the
// reservation) and ends CANCELLED (+order.cancelled). It is idempotent on
// idempotency_key.
func (s *Saga) PlaceOrder(ctx context.Context, userID, idempotencyKey string) (*Result, error) {
	if idempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}

	// 1. Idempotency: a retried PlaceOrder with the same key returns the existing
	// order, never a second one.
	if existing, err := s.q.GetOrderByIdempotencyKey(ctx, &idempotencyKey); err == nil {
		return &Result{OrderID: uuidStr(existing.ID), Status: order.Status(existing.Status)}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, s.internal(ctx, "lookup order by key", err)
	}

	// 2. Fetch the cart.
	cartResp, err := s.carts.GetCart(ctx, &cartv1.GetCartRequest{UserId: userID})
	if err != nil {
		return nil, s.internal(ctx, "get cart", err)
	}
	cartItems := cartResp.GetCart().GetItems()
	if len(cartItems) == 0 {
		return nil, status.Error(codes.FailedPrecondition, ErrEmptyCart.Error())
	}

	// 3. Resolve prices from the Product service (authoritative) and snapshot them.
	var lines []lineItem
	var total int64
	for _, ci := range cartItems {
		p, err := s.products.GetProduct(ctx, &productv1.GetProductRequest{ProductId: ci.GetProductId()})
		if err != nil {
			// A product in the cart no longer exists/loads — can't price the order.
			return nil, status.Errorf(codes.FailedPrecondition, "product %s unavailable", ci.GetProductId())
		}
		prod := p.GetProduct()
		lines = append(lines, lineItem{prod.GetId(), prod.GetName(), prod.GetPriceCents(), ci.GetQuantity()})
		total += prod.GetPriceCents() * int64(ci.GetQuantity())
	}

	// 4. Create the order PENDING + items in ONE transaction. id == reservation_id.
	orderID := uuid.New()
	if err := s.createOrder(ctx, orderID, userUUID, idempotencyKey, total, lines); err != nil {
		// A concurrent PlaceOrder with the same key won the unique race — return it.
		if isUniqueViolation(err) {
			if existing, gerr := s.q.GetOrderByIdempotencyKey(ctx, &idempotencyKey); gerr == nil {
				return &Result{OrderID: uuidStr(existing.ID), Status: order.Status(existing.Status)}, nil
			}
		}
		return nil, s.internal(ctx, "create order", err)
	}

	ev := orderEvent{OrderID: orderID.String(), UserID: userID, TotalCents: total}

	// 5. Reserve stock under the order id.
	reserveItems := make([]*productv1.ReserveItem, 0, len(lines))
	for _, l := range lines {
		reserveItems = append(reserveItems, &productv1.ReserveItem{ProductId: l.productID, Quantity: l.quantity})
	}
	reserve, err := s.products.ReserveStock(ctx, &productv1.ReserveStockRequest{
		ReservationId: orderID.String(),
		Items:         reserveItems,
	})
	if err != nil || !reserve.GetSuccess() {
		// Couldn't reserve (insufficient stock or service error): nothing to undo,
		// just cancel.
		return s.cancel(ctx, orderID, ev), nil
	}
	if err := s.setStatus(ctx, orderID, order.StatusStockReserved, "", nil); err != nil {
		return nil, s.internal(ctx, "set STOCK_RESERVED", err)
	}

	// 6. Take payment (idempotent on the same key).
	if err := s.setStatus(ctx, orderID, order.StatusPaymentPending, "", nil); err != nil {
		return nil, s.internal(ctx, "set PAYMENT_PENDING", err)
	}
	pay, err := s.payments.CreatePayment(ctx, &paymentv1.CreatePaymentRequest{
		OrderId:        orderID.String(),
		AmountCents:    total,
		Currency:       "NGN",
		IdempotencyKey: idempotencyKey,
	})
	if err != nil || pay.GetStatus() != "succeeded" {
		// 7b. Declined or payment error -> COMPENSATE: release the reservation, cancel.
		if _, rerr := s.products.ReleaseStock(ctx, &productv1.ReleaseStockRequest{ReservationId: orderID.String()}); rerr != nil {
			s.log.ErrorContext(ctx, "release stock during compensation", "err", rerr, "order_id", orderID)
		}
		_ = s.setStatus(ctx, orderID, order.StatusPaymentFailed, "", nil)
		return s.cancel(ctx, orderID, ev), nil
	}

	// 7a. Payment succeeded -> commit the reservation into a real decrement.
	if _, cerr := s.products.CommitStock(ctx, &productv1.CommitStockRequest{ReservationId: orderID.String()}); cerr != nil {
		// Payment took, but commit failed: log loudly. The order is paid; a real
		// system reconciles the commit. We proceed to mark it paid/confirmed.
		s.log.ErrorContext(ctx, "commit stock after payment", "err", cerr, "order_id", orderID)
	}

	paymentID, _ := uuid.Parse(pay.GetPaymentId())
	paidEv := ev
	paidEv.Status = string(order.StatusPaid)
	if err := s.markPaid(ctx, orderID, paymentID, paidEv); err != nil {
		return nil, s.internal(ctx, "mark PAID", err)
	}

	// Clear the cart (best-effort; the order is already paid).
	if _, cerr := s.carts.ClearCart(ctx, &cartv1.ClearCartRequest{UserId: userID}); cerr != nil {
		s.log.ErrorContext(ctx, "clear cart after order", "err", cerr, "user_id", userID)
	}

	confirmEv := ev
	confirmEv.Status = string(order.StatusConfirmed)
	if err := s.setStatus(ctx, orderID, order.StatusConfirmed, "order.confirmed", &confirmEv); err != nil {
		return nil, s.internal(ctx, "set CONFIRMED", err)
	}

	s.log.InfoContext(ctx, "order confirmed", "order_id", orderID, "total_cents", total)
	return &Result{OrderID: orderID.String(), Status: order.StatusConfirmed}, nil
}

// Cancel handles an explicit CancelOrder request. A PAID/CONFIRMED order can't be
// cancelled (the money moved); an already-CANCELLED one is a no-op. Otherwise it
// releases any held reservation (safe/idempotent) and ends CANCELLED.
func (s *Saga) Cancel(ctx context.Context, orderID string) (order.Status, error) {
	oid, err := uuid.Parse(orderID)
	if err != nil {
		return "", status.Error(codes.InvalidArgument, "order_id must be a UUID")
	}
	o, err := s.q.GetOrder(ctx, pgUUID(oid))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", status.Error(codes.NotFound, "order not found")
		}
		return "", s.internal(ctx, "get order", err)
	}

	switch order.Status(o.Status) {
	case order.StatusPaid, order.StatusConfirmed:
		return "", status.Error(codes.FailedPrecondition, "cannot cancel a paid order")
	case order.StatusCancelled:
		return order.StatusCancelled, nil // idempotent
	}

	// Release any reservation (ReleaseStock is a no-op if nothing was reserved).
	if _, rerr := s.products.ReleaseStock(ctx, &productv1.ReleaseStockRequest{ReservationId: orderID}); rerr != nil {
		s.log.ErrorContext(ctx, "release stock during cancel", "err", rerr, "order_id", orderID)
	}
	ev := orderEvent{OrderID: orderID, UserID: uuidStr(o.UserID), TotalCents: o.TotalCents, Status: string(order.StatusCancelled)}
	if err := s.setStatus(ctx, oid, order.StatusCancelled, "order.cancelled", &ev); err != nil {
		return "", s.internal(ctx, "set CANCELLED", err)
	}
	return order.StatusCancelled, nil
}

// cancel sets CANCELLED and writes order.cancelled in one tx.
func (s *Saga) cancel(ctx context.Context, orderID uuid.UUID, ev orderEvent) *Result {
	cancelledEv := ev
	cancelledEv.Status = string(order.StatusCancelled)
	if err := s.setStatus(ctx, orderID, order.StatusCancelled, "order.cancelled", &cancelledEv); err != nil {
		s.log.ErrorContext(ctx, "set CANCELLED", "err", err, "order_id", orderID)
	}
	return &Result{OrderID: orderID.String(), Status: order.StatusCancelled}
}

// createOrder inserts the order + items in one transaction.
func (s *Saga) createOrder(ctx context.Context, orderID, userUUID uuid.UUID, key string, total int64, lines []lineItem) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	if _, err := q.CreateOrder(ctx, db.CreateOrderParams{
		ID:             pgUUID(orderID),
		UserID:         pgUUID(userUUID),
		Status:         string(order.StatusPending),
		TotalCents:     total,
		Currency:       "NGN",
		ReservationID:  pgUUID(orderID),
		IdempotencyKey: &key,
	}); err != nil {
		return err
	}
	for _, l := range lines {
		pid, err := uuid.Parse(l.productID)
		if err != nil {
			return fmt.Errorf("bad product id %q: %w", l.productID, err)
		}
		if err := q.CreateOrderItem(ctx, db.CreateOrderItemParams{
			OrderID:    pgUUID(orderID),
			ProductID:  pgUUID(pid),
			Name:       l.name,
			PriceCents: l.priceCents,
			Quantity:   l.quantity,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// setStatus updates the order status and (optionally) writes an outbox event in
// the SAME transaction — so a state change and its event are atomic.
func (s *Saga) setStatus(ctx context.Context, orderID uuid.UUID, to order.Status, topic string, ev *orderEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	if _, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{ID: pgUUID(orderID), Status: string(to)}); err != nil {
		return err
	}
	if topic != "" && ev != nil {
		if err := writeOutbox(ctx, q, topic, ev); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// markPaid sets payment_id + PAID and writes order.paid, all in one tx.
func (s *Saga) markPaid(ctx context.Context, orderID, paymentID uuid.UUID, ev orderEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	if _, err := q.SetOrderPaymentAndStatus(ctx, db.SetOrderPaymentAndStatusParams{
		ID:        pgUUID(orderID),
		PaymentID: pgUUID(paymentID),
		Status:    string(order.StatusPaid),
	}); err != nil {
		return err
	}
	if err := writeOutbox(ctx, q, "order.paid", &ev); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func writeOutbox(ctx context.Context, q *db.Queries, topic string, ev *orderEvent) error {
	ev.EventID = uuid.NewString()
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return q.InsertOutbox(ctx, db.InsertOutboxParams{Topic: topic, Payload: payload})
}

func (s *Saga) internal(ctx context.Context, msg string, err error) error {
	s.log.ErrorContext(ctx, msg, "err", err)
	return status.Error(codes.Internal, "internal error")
}

func pgUUID(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }

func uuidStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
