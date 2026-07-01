// Package saga orchestrates turning a cart into a paid order. It is the heart of
// the system: a sequence of cross-service calls (Cart, Product, Payment) with a
// state machine and COMPENSATING actions when a later step fails. Events are
// written to the outbox in the same DB transaction as the state change.
package saga

import (
	"context"
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

	"github.com/menawar/ecommerce-platform/pkg/events"
	cartv1 "github.com/menawar/ecommerce-platform/proto/cart/v1"
	paymentv1 "github.com/menawar/ecommerce-platform/proto/payment/v1"
	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
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
	users    userv1.UserServiceClient
	log      *slog.Logger
}

func New(
	pool *pgxpool.Pool,
	carts cartv1.CartServiceClient,
	products productv1.ProductServiceClient,
	payments paymentv1.PaymentServiceClient,
	users userv1.UserServiceClient,
	log *slog.Logger,
) *Saga {
	return &Saga{pool: pool, q: db.New(pool), carts: carts, products: products, payments: payments, users: users, log: log}
}

// Result is what PlaceOrder reports back. In the async flow PlaceOrder returns at
// PAYMENT_PENDING with an AuthorizationURL; the terminal status arrives later via
// the payment.* event consumer.
type Result struct {
	OrderID          string
	Status           order.Status
	AuthorizationURL string
}

// lineItem is a priced cart line, snapshotted from the Product service.
type lineItem struct {
	productID  string
	name       string
	priceCents int64
	quantity   int32
}

// orderEvent is the DATA payload of order.* events. It's wrapped in an
// events.Envelope (which supplies event_id/occurred_at/version) before going to
// the outbox, so consumers dedupe on the envelope's event_id.
type orderEvent struct {
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
func (s *Saga) PlaceOrder(ctx context.Context, userID, idempotencyKey, addressID, shippingMethodID string) (*Result, error) {
	if idempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	smUUID, err := uuid.Parse(shippingMethodID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "shipping_method_id must be a UUID")
	}
	if _, err := uuid.Parse(addressID); err != nil {
		return nil, status.Error(codes.InvalidArgument, "address_id must be a UUID")
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
	var subtotal int64
	for _, ci := range cartItems {
		p, err := s.products.GetProduct(ctx, &productv1.GetProductRequest{ProductId: ci.GetProductId()})
		if err != nil {
			// A product in the cart no longer exists/loads — can't price the order.
			return nil, status.Errorf(codes.FailedPrecondition, "product %s unavailable", ci.GetProductId())
		}
		prod := p.GetProduct()
		lines = append(lines, lineItem{prod.GetId(), prod.GetName(), prod.GetPriceCents(), ci.GetQuantity()})
		subtotal += prod.GetPriceCents() * int64(ci.GetQuantity())
	}

	// 3a. Resolve the chosen shipping method (same DB) — must exist and be active.
	sm, err := s.q.GetShippingMethod(ctx, pgUUID(smUUID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.FailedPrecondition, "shipping method unavailable")
		}
		return nil, s.internal(ctx, "get shipping method", err)
	}
	if !sm.Active {
		return nil, status.Error(codes.FailedPrecondition, "shipping method unavailable")
	}

	// 3b. Snapshot the chosen address from the User service (db-per-service: the
	// order service can't read userdb). GetAddress is scoped by user_id, so a
	// missing/not-owned id comes back NotFound -> treat as a bad checkout input.
	addrResp, err := s.users.GetAddress(ctx, &userv1.GetAddressRequest{UserId: userID, AddressId: addressID})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, status.Error(codes.FailedPrecondition, "address unavailable")
		}
		return nil, s.internal(ctx, "get address", err)
	}

	// 4. Create the order PENDING + items in ONE transaction. id == reservation_id.
	// total = subtotal + shipping.
	total := subtotal + sm.PriceCents
	orderID := uuid.New()
	ship := shipSnapshot{methodID: smUUID, methodName: sm.Name, cents: sm.PriceCents, addr: addrResp.GetAddress()}
	if err := s.createOrder(ctx, orderID, userUUID, idempotencyKey, total, lines, ship); err != nil {
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

	// 6. Resolve the customer's email authoritatively from the User service — the
	// PSP needs it to start the charge and address the receipt. (We never trust a
	// request-supplied email for this.)
	usr, err := s.users.GetUser(ctx, &userv1.GetUserRequest{UserId: userID})
	if err != nil {
		// Can't initialize a charge without an email; nothing is committed, so
		// COMPENSATE: release the reservation and cancel.
		s.log.ErrorContext(ctx, "resolve user email", "err", err, "order_id", orderID)
		if _, rerr := s.products.ReleaseStock(ctx, &productv1.ReleaseStockRequest{ReservationId: orderID.String()}); rerr != nil {
			s.log.ErrorContext(ctx, "release stock during compensation", "err", rerr, "order_id", orderID)
		}
		return s.cancel(ctx, orderID, ev), nil
	}

	// 7. Initialize payment with the PSP (idempotent on the same key). This does NOT
	// charge — it returns a hosted-checkout URL. The terminal outcome arrives later
	// via the payment.succeeded/payment.failed event, handled by the resume consumer.
	pay, err := s.payments.InitializePayment(ctx, &paymentv1.InitializePaymentRequest{
		OrderId:        orderID.String(),
		AmountCents:    total,
		Currency:       "NGN",
		IdempotencyKey: idempotencyKey,
		Email:          usr.GetEmail(),
	})
	if err != nil {
		// Couldn't even start the charge -> COMPENSATE: release the reservation, cancel.
		if _, rerr := s.products.ReleaseStock(ctx, &productv1.ReleaseStockRequest{ReservationId: orderID.String()}); rerr != nil {
			s.log.ErrorContext(ctx, "release stock during compensation", "err", rerr, "order_id", orderID)
		}
		return s.cancel(ctx, orderID, ev), nil
	}

	// 8. Record the initialized payment + its authorization URL as the order enters
	// PAYMENT_PENDING. The saga PAUSES here: the customer authorizes at the PSP, and
	// the webhook-driven consumer (payment.* events) resumes it to CONFIRMED/CANCELLED.
	paymentID, _ := uuid.Parse(pay.GetPaymentId())
	if _, err := s.q.MarkOrderPaymentPending(ctx, db.MarkOrderPaymentPendingParams{
		ID:               pgUUID(orderID),
		PaymentID:        pgtype.UUID{Bytes: paymentID, Valid: true},
		AuthorizationUrl: pay.GetAuthorizationUrl(),
	}); err != nil {
		return nil, s.internal(ctx, "mark PAYMENT_PENDING", err)
	}

	s.log.InfoContext(ctx, "order awaiting payment", "order_id", orderID, "total_cents", total)
	return &Result{
		OrderID:          orderID.String(),
		Status:           order.StatusPaymentPending,
		AuthorizationURL: pay.GetAuthorizationUrl(),
	}, nil
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

	switch st := order.Status(o.Status); {
	case st == order.StatusCancelled:
		return order.StatusCancelled, nil // idempotent
	case st == order.StatusPaid || st.IsPostPayment():
		// Paid or already confirmed/shipped/delivered: the money moved, so the only
		// way back is a refund (Phase 11.3), never a cancel.
		return "", status.Error(codes.FailedPrecondition, "cannot cancel a paid order")
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

// maxTrackingLen bounds the admin-supplied tracking number.
const maxTrackingLen = 128

// MarkShipped moves a CONFIRMED order to SHIPPED (recording an optional tracking
// number), emitting order.shipped in the same tx. Admin-only (gateway-enforced).
func (s *Saga) MarkShipped(ctx context.Context, orderID, tracking string) (db.Order, error) {
	if len(tracking) > maxTrackingLen {
		return db.Order{}, status.Errorf(codes.InvalidArgument, "tracking number too long (max %d)", maxTrackingLen)
	}
	return s.advanceFulfillment(ctx, orderID, order.StatusShipped, tracking)
}

// MarkDelivered moves a SHIPPED order to DELIVERED (terminal), emitting
// order.delivered in the same tx.
func (s *Saga) MarkDelivered(ctx context.Context, orderID string) (db.Order, error) {
	return s.advanceFulfillment(ctx, orderID, order.StatusDelivered, "")
}

// advanceFulfillment guards a CONFIRMED->SHIPPED->DELIVERED step and applies it +
// its event atomically. A missing order is NotFound; an illegal step (e.g. ship a
// PENDING order, or deliver one that isn't shipped) is FailedPrecondition.
func (s *Saga) advanceFulfillment(ctx context.Context, orderID string, to order.Status, tracking string) (db.Order, error) {
	oid, err := uuid.Parse(orderID)
	if err != nil {
		return db.Order{}, status.Error(codes.InvalidArgument, "order_id must be a UUID")
	}
	o, err := s.q.GetOrder(ctx, pgUUID(oid))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Order{}, status.Error(codes.NotFound, "order not found")
		}
		return db.Order{}, s.internal(ctx, "get order", err)
	}
	if !order.Status(o.Status).CanTransitionTo(to) {
		return db.Order{}, status.Errorf(codes.FailedPrecondition, "order in %s cannot move to %s", o.Status, to)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.Order{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	var updated db.Order
	var topic string
	switch to {
	case order.StatusShipped:
		updated, err = q.MarkOrderShipped(ctx, db.MarkOrderShippedParams{ID: pgUUID(oid), TrackingNumber: tracking})
		topic = "order.shipped"
	case order.StatusDelivered:
		updated, err = q.MarkOrderDelivered(ctx, pgUUID(oid))
		topic = "order.delivered"
	default:
		return db.Order{}, s.internal(ctx, "advanceFulfillment", fmt.Errorf("unsupported target %s", to))
	}
	if err != nil {
		// The UPDATE has a status precondition (compare-and-set): no rows means the
		// order left the source state between our read and the write (a concurrent
		// mark). Treat as a lost race, not an internal error — and no event is
		// written, so exactly one order.shipped/delivered ever fires.
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Order{}, status.Errorf(codes.FailedPrecondition, "order is no longer eligible to move to %s", to)
		}
		return db.Order{}, s.internal(ctx, "update fulfillment status", err)
	}
	ev := orderEvent{OrderID: orderID, UserID: uuidStr(o.UserID), TotalCents: o.TotalCents, Status: string(to)}
	if err := writeOutbox(ctx, q, topic, ev); err != nil {
		return db.Order{}, s.internal(ctx, "write fulfillment event", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Order{}, s.internal(ctx, "commit fulfillment", err)
	}
	s.log.InfoContext(ctx, "order fulfillment advanced", "order_id", orderID, "status", to)
	return updated, nil
}

// paymentEventData mirrors the payment service's payment.succeeded/payment.failed
// payload. We only need the order id to correlate; the topic carries the outcome.
type paymentEventData struct {
	OrderID string `json:"order_id"`
}

// HandlePaymentEvent is the order service's consumer callback for the EVENTS
// stream. It resumes the matching order when its payment settles; every other
// topic (order.*, user.*) is ignored. It MUST be idempotent — see Resume.
func (s *Saga) HandlePaymentEvent(ctx context.Context, env events.Envelope) error {
	var paid bool
	switch env.Topic {
	case "payment.succeeded":
		paid = true
	case "payment.failed":
		paid = false
	default:
		return nil // not a payment outcome — nothing to do
	}
	data, err := events.DataAs[paymentEventData](env)
	if err != nil {
		return fmt.Errorf("decode payment event: %w", err)
	}
	if data.OrderID == "" {
		return nil // can't correlate without an order id
	}
	return s.Resume(ctx, data.OrderID, paid)
}

// Resume continues a PAYMENT_PENDING order once its payment settles. On success it
// commits the reservation, marks the order PAID (+order.paid), clears the cart, and
// CONFIRMS it (+order.confirmed); on failure it COMPENSATES — releases the
// reservation and CANCELS (+order.cancelled).
//
// It is idempotent for at-least-once delivery: an order already terminal is a
// no-op, a duplicate success re-runs the (idempotent) commit/confirm steps, and a
// "failed" arriving after PAID is ignored (a paid order is never unpaid).
func (s *Saga) Resume(ctx context.Context, orderID string, paid bool) error {
	oid, err := uuid.Parse(orderID)
	if err != nil {
		return status.Error(codes.InvalidArgument, "order_id must be a UUID")
	}
	o, err := s.q.GetOrder(ctx, pgUUID(oid))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return status.Error(codes.NotFound, "order not found")
		}
		return s.internal(ctx, "get order", err)
	}

	st := order.Status(o.Status)
	if st.IsPostPayment() {
		return nil // already settled (confirmed/shipped/delivered/cancelled) — idempotent
	}
	ev := orderEvent{OrderID: orderID, UserID: uuidStr(o.UserID), TotalCents: o.TotalCents}

	if !paid {
		if st == order.StatusPaid {
			return nil // a 'failed' after we recorded PAID — can't unpay; ignore
		}
		// Declined -> COMPENSATE: release the reservation, cancel.
		if _, rerr := s.products.ReleaseStock(ctx, &productv1.ReleaseStockRequest{ReservationId: orderID}); rerr != nil {
			s.log.ErrorContext(ctx, "release stock during compensation", "err", rerr, "order_id", orderID)
		}
		s.cancel(ctx, oid, ev)
		return nil
	}

	// Succeeded -> commit the reservation into a real decrement (idempotent on the
	// reservation id, so a redelivered event is safe).
	if _, cerr := s.products.CommitStock(ctx, &productv1.CommitStockRequest{ReservationId: orderID}); cerr != nil {
		s.log.ErrorContext(ctx, "commit stock after payment", "err", cerr, "order_id", orderID)
	}

	paidEv := ev
	paidEv.Status = string(order.StatusPaid)
	if err := s.markPaid(ctx, oid, uuid.UUID(o.PaymentID.Bytes), paidEv); err != nil {
		return s.internal(ctx, "mark PAID", err)
	}

	// Clear the cart (best-effort; the order is already paid).
	if _, cerr := s.carts.ClearCart(ctx, &cartv1.ClearCartRequest{UserId: uuidStr(o.UserID)}); cerr != nil {
		s.log.ErrorContext(ctx, "clear cart after order", "err", cerr, "user_id", uuidStr(o.UserID))
	}

	confirmEv := ev
	confirmEv.Status = string(order.StatusConfirmed)
	if err := s.setStatus(ctx, oid, order.StatusConfirmed, "order.confirmed", &confirmEv); err != nil {
		return s.internal(ctx, "set CONFIRMED", err)
	}
	s.log.InfoContext(ctx, "order confirmed", "order_id", orderID, "total_cents", o.TotalCents)
	return nil
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

// shipSnapshot is the shipping method + address copy persisted onto the order at
// checkout, so later edits to the method catalog or the user's address book never
// rewrite a past order.
type shipSnapshot struct {
	methodID   uuid.UUID
	methodName string
	cents      int64
	addr       *userv1.Address
}

// createOrder inserts the order + items in one transaction.
func (s *Saga) createOrder(ctx context.Context, orderID, userUUID uuid.UUID, key string, total int64, lines []lineItem, ship shipSnapshot) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	a := ship.addr
	if _, err := q.CreateOrder(ctx, db.CreateOrderParams{
		ID:                 pgUUID(orderID),
		UserID:             pgUUID(userUUID),
		Status:             string(order.StatusPending),
		TotalCents:         total,
		Currency:           "NGN",
		ReservationID:      pgUUID(orderID),
		IdempotencyKey:     &key,
		ShippingMethodID:   pgUUID(ship.methodID),
		ShippingMethodName: ship.methodName,
		ShippingCents:      ship.cents,
		ShipRecipient:      a.GetRecipient(),
		ShipPhone:          a.GetPhone(),
		ShipLine1:          a.GetLine1(),
		ShipLine2:          a.GetLine2(),
		ShipCity:           a.GetCity(),
		ShipState:          a.GetState(),
		ShipPostalCode:     a.GetPostalCode(),
		ShipCountry:        a.GetCountry(),
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
		if err := writeOutbox(ctx, q, topic, *ev); err != nil {
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
	if err := writeOutbox(ctx, q, "order.paid", ev); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func writeOutbox(ctx context.Context, q *db.Queries, topic string, data orderEvent) error {
	env, err := events.New(topic, data)
	if err != nil {
		return err
	}
	payload, err := env.Marshal()
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
