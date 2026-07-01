// Package server implements payment.v1.PaymentService. The heart of it is
// idempotent CreatePayment: a retried or concurrent request with the same
// idempotency_key must return the original payment and charge AT MOST ONCE.
package server

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/menawar/ecommerce-platform/pkg/events"
	paymentv1 "github.com/menawar/ecommerce-platform/proto/payment/v1"
	"github.com/menawar/ecommerce-platform/services/payment/internal/db"
	"github.com/menawar/ecommerce-platform/services/payment/internal/provider"
)

const (
	statusSucceeded = "succeeded"
	statusFailed    = "failed"
	statusPending   = "pending"
)

type Server struct {
	paymentv1.UnimplementedPaymentServiceServer
	pool         *pgxpool.Pool
	q            *db.Queries
	async        provider.AsyncProvider // redirect+webhook charge (InitializePayment)
	providerName string                 // provider name stored on payment rows
	log          *slog.Logger
}

func NewServer(pool *pgxpool.Pool, log *slog.Logger) *Server {
	return &Server{pool: pool, q: db.New(pool), log: log}
}

// WithAsync configures the asynchronous (redirect+webhook) provider used by
// InitializePayment and the webhook handler. name is recorded on the payment row.
func (s *Server) WithAsync(name string, a provider.AsyncProvider) *Server {
	s.providerName = name
	s.async = a
	return s
}

// InitializePayment starts an asynchronous charge with the redirect-based PSP. It
// claims the idempotency key with a PENDING row, asks
// the provider for an authorization URL, and saves the provider reference so a
// later webhook can find this payment. The terminal status arrives via the webhook,
// NOT here — this returns 'pending'.
func (s *Server) InitializePayment(ctx context.Context, req *paymentv1.InitializePaymentRequest) (*paymentv1.InitializePaymentResponse, error) {
	if s.async == nil {
		return nil, status.Error(codes.Unimplemented, "async payment provider not configured")
	}
	orderID, err := uuid.Parse(req.GetOrderId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "order_id must be a UUID")
	}
	if req.GetIdempotencyKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if req.GetAmountCents() < 0 {
		return nil, status.Error(codes.InvalidArgument, "amount_cents must be non-negative")
	}

	// Idempotent replay returns the original payment. The authorization_url isn't
	// re-derivable from the provider, so a pure retry returns it empty — the order
	// saga persists the URL on the first call.
	if existing, err := s.q.GetPaymentByIdempotencyKey(ctx, req.GetIdempotencyKey()); err == nil {
		return &paymentv1.InitializePaymentResponse{PaymentId: uuidStr(existing.ID), Status: existing.Status}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, s.internal(ctx, "lookup payment by key", err)
	}

	// Claim the key with a PENDING row before calling the PSP — the UNIQUE
	// constraint makes a concurrent retry read the winner instead of charging twice.
	pending, err := s.q.CreatePayment(ctx, db.CreatePaymentParams{
		OrderID:        pgtype.UUID{Bytes: orderID, Valid: true},
		AmountCents:    req.GetAmountCents(),
		Currency:       currencyOrDefault(req.GetCurrency()),
		Status:         statusPending,
		Provider:       s.providerName,
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		if isUniqueViolation(err) {
			if existing, gerr := s.q.GetPaymentByIdempotencyKey(ctx, req.GetIdempotencyKey()); gerr == nil {
				return &paymentv1.InitializePaymentResponse{PaymentId: uuidStr(existing.ID), Status: existing.Status}, nil
			}
		}
		return nil, s.internal(ctx, "claim payment", err)
	}

	// We own this key: start the PSP transaction exactly once. Use the payment id as
	// the reference so the webhook can correlate the callback back to this row.
	authURL, providerRef, err := s.async.Initialize(ctx, req.GetAmountCents(), pending.Currency, uuidStr(pending.ID), req.GetEmail())
	if err != nil {
		return nil, s.internal(ctx, "initialize payment", err)
	}
	if _, err := s.q.UpdatePaymentResult(ctx, db.UpdatePaymentResultParams{
		ID:          pending.ID,
		Status:      statusPending, // still pending; the webhook finalizes it
		ProviderRef: &providerRef,
	}); err != nil {
		return nil, s.internal(ctx, "save provider ref", err)
	}

	return &paymentv1.InitializePaymentResponse{
		PaymentId:        uuidStr(pending.ID),
		Status:           statusPending,
		AuthorizationUrl: authURL,
	}, nil
}

// ConfirmPayment is driven by the webhook. It RE-VERIFIES the transaction with the
// provider (the authoritative check — the webhook body is never trusted on its
// own), then atomically transitions the payment and writes a
// payment.succeeded/payment.failed event to the outbox. Idempotent: a payment
// already in a terminal state is a no-op, since webhooks are at-least-once.
func (s *Server) ConfirmPayment(ctx context.Context, providerRef string) error {
	p, err := s.q.GetPaymentByProviderRef(ctx, &providerRef)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return status.Error(codes.NotFound, "payment not found for reference")
		}
		return s.internal(ctx, "lookup payment by provider ref", err)
	}
	if p.Status == statusSucceeded || p.Status == statusFailed {
		return nil // already finalized — idempotent replay
	}

	verified, err := s.async.Verify(ctx, providerRef)
	if err != nil {
		return s.internal(ctx, "verify payment", err)
	}
	if verified == provider.StatusPending {
		return nil // not finished authorizing; a later webhook/verify will settle it
	}

	topic := "payment.succeeded"
	if verified == statusFailed {
		topic = "payment.failed"
	}

	// The status change and its event commit together (transactional outbox), so a
	// confirmed payment can never be marked paid without its event, or vice versa.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return s.internal(ctx, "begin tx", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	if _, err := q.UpdatePaymentResult(ctx, db.UpdatePaymentResultParams{
		ID:          p.ID,
		Status:      verified,
		ProviderRef: &providerRef,
	}); err != nil {
		return s.internal(ctx, "update payment", err)
	}
	if err := writeOutbox(ctx, q, topic, paymentEvent{
		PaymentID:   uuidStr(p.ID),
		OrderID:     uuidStr(p.OrderID),
		AmountCents: p.AmountCents,
		Status:      verified,
	}); err != nil {
		return s.internal(ctx, "write outbox", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return s.internal(ctx, "commit", err)
	}
	s.log.InfoContext(ctx, "payment finalized via webhook", "payment_id", uuidStr(p.ID), "status", verified)
	return nil
}

// paymentEvent is the payload of payment.succeeded / payment.failed. The order saga
// consumes it (keyed by order_id) to resume from PAYMENT_PENDING.
type paymentEvent struct {
	PaymentID   string `json:"payment_id"`
	OrderID     string `json:"order_id"`
	AmountCents int64  `json:"amount_cents"`
	Status      string `json:"status"`
}

func writeOutbox(ctx context.Context, q *db.Queries, topic string, data paymentEvent) error {
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

const statusRefunded = "refunded"

// RefundPayment reverses a succeeded charge synchronously. It is idempotent: an
// already-refunded payment returns success without calling the provider again.
// (No event is emitted here — the Order service, which drives refunds, emits
// order.refunded once it marks the order REFUNDED.)
func (s *Server) RefundPayment(ctx context.Context, req *paymentv1.RefundPaymentRequest) (*paymentv1.RefundPaymentResponse, error) {
	if s.async == nil {
		return nil, status.Error(codes.Unimplemented, "async payment provider not configured")
	}
	id, err := uuid.Parse(req.GetPaymentId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "payment_id must be a UUID")
	}
	p, err := s.q.GetPaymentByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "payment not found")
		}
		return nil, s.internal(ctx, "get payment", err)
	}
	if p.Status == statusRefunded {
		return &paymentv1.RefundPaymentResponse{Status: statusRefunded}, nil // idempotent
	}
	if p.Status != statusSucceeded {
		return nil, status.Errorf(codes.FailedPrecondition, "payment is %s, not refundable", p.Status)
	}
	if p.ProviderRef == nil || *p.ProviderRef == "" {
		return nil, s.internal(ctx, "refund payment", errors.New("payment has no provider reference"))
	}

	// Reverse the funds at the PSP (synchronous). Note: a genuinely-concurrent
	// double refund could call the provider twice — acceptable for the admin-driven
	// flow; the idempotent status check above collapses the common retry case.
	if err := s.async.Refund(ctx, *p.ProviderRef, p.AmountCents); err != nil {
		return nil, s.internal(ctx, "provider refund", err)
	}
	// Atomic compare-and-set (WHERE status='succeeded'); ErrNoRows means a concurrent
	// refund already flipped it — still success, don't error.
	if _, err := s.q.MarkPaymentRefunded(ctx, p.ID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, s.internal(ctx, "mark refunded", err)
	}
	s.log.InfoContext(ctx, "payment refunded", "payment_id", uuidStr(p.ID))
	return &paymentv1.RefundPaymentResponse{Status: statusRefunded}, nil
}

func (s *Server) GetPayment(ctx context.Context, req *paymentv1.GetPaymentRequest) (*paymentv1.GetPaymentResponse, error) {
	id, err := uuid.Parse(req.GetPaymentId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "payment_id must be a UUID")
	}
	p, err := s.q.GetPaymentByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "payment not found")
		}
		return nil, s.internal(ctx, "get payment", err)
	}
	return &paymentv1.GetPaymentResponse{Payment: toProto(p)}, nil
}

func (s *Server) internal(ctx context.Context, msg string, err error) error {
	s.log.ErrorContext(ctx, msg, "err", err)
	return status.Error(codes.Internal, "internal error")
}

func currencyOrDefault(c string) string {
	if c == "" {
		return "NGN"
	}
	return c
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func uuidStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

func toProto(p db.Payment) *paymentv1.Payment {
	ref := ""
	if p.ProviderRef != nil {
		ref = *p.ProviderRef
	}
	var created int64
	if p.CreatedAt.Valid {
		created = p.CreatedAt.Time.Unix()
	}
	return &paymentv1.Payment{
		Id:          uuidStr(p.ID),
		OrderId:     uuidStr(p.OrderID),
		AmountCents: p.AmountCents,
		Currency:    p.Currency,
		Status:      p.Status,
		Provider:    p.Provider,
		ProviderRef: ref,
		CreatedAt:   created,
	}
}
