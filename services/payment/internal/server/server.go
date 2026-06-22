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
	q        *db.Queries
	provider provider.Provider
	log      *slog.Logger
}

func NewServer(pool *pgxpool.Pool, prov provider.Provider, log *slog.Logger) *Server {
	return &Server{q: db.New(pool), provider: prov, log: log}
}

func (s *Server) CreatePayment(ctx context.Context, req *paymentv1.CreatePaymentRequest) (*paymentv1.CreatePaymentResponse, error) {
	orderID, err := uuid.Parse(req.GetOrderId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "order_id must be a UUID")
	}
	if req.GetIdempotencyKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}
	if req.GetAmountCents() < 0 {
		return nil, status.Error(codes.InvalidArgument, "amount_cents must be non-negative")
	}

	// Fast path: this key was already processed -> return the original result. This
	// is the common retry case (the saga re-calls after a timeout).
	if existing, err := s.q.GetPaymentByIdempotencyKey(ctx, req.GetIdempotencyKey()); err == nil {
		return &paymentv1.CreatePaymentResponse{PaymentId: uuidStr(existing.ID), Status: existing.Status}, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, s.internal(ctx, "lookup payment by key", err)
	}

	// Claim the key by inserting a PENDING row BEFORE charging. The UNIQUE
	// constraint means a concurrent request with the same key loses here and reads
	// the winner's row instead of charging a second time.
	pending, err := s.q.CreatePayment(ctx, db.CreatePaymentParams{
		OrderID:        pgtype.UUID{Bytes: orderID, Valid: true},
		AmountCents:    req.GetAmountCents(),
		Currency:       currencyOrDefault(req.GetCurrency()),
		Status:         statusPending,
		Provider:       provider.NameMock,
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		if isUniqueViolation(err) {
			existing, gerr := s.q.GetPaymentByIdempotencyKey(ctx, req.GetIdempotencyKey())
			if gerr == nil {
				return &paymentv1.CreatePaymentResponse{PaymentId: uuidStr(existing.ID), Status: existing.Status}, nil
			}
		}
		return nil, s.internal(ctx, "claim payment", err)
	}

	// We own this key: charge exactly once.
	ref, chargeErr := s.provider.Charge(ctx, req.GetAmountCents(), pending.Currency, req.GetOrderId())
	newStatus := statusSucceeded
	var providerRef *string
	switch {
	case chargeErr == nil:
		providerRef = &ref
	case errors.Is(chargeErr, provider.ErrDeclined):
		newStatus = statusFailed // a normal decline — the saga compensates
	default:
		// Unexpected provider/infra error: leave the row pending and surface it.
		return nil, s.internal(ctx, "charge", chargeErr)
	}

	updated, err := s.q.UpdatePaymentResult(ctx, db.UpdatePaymentResultParams{
		ID:          pending.ID,
		Status:      newStatus,
		ProviderRef: providerRef,
	})
	if err != nil {
		return nil, s.internal(ctx, "update payment result", err)
	}

	s.log.InfoContext(ctx, "processed payment", "payment_id", uuidStr(updated.ID), "status", updated.Status)
	return &paymentv1.CreatePaymentResponse{PaymentId: uuidStr(updated.ID), Status: updated.Status}, nil
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
