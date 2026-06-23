// Package server implements order.v1.OrderService. PlaceOrder/CancelOrder delegate
// to the saga; GetOrder/ListOrders are straight reads mapped to proto.
package server

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
	"github.com/menawar/ecommerce-platform/services/order/internal/db"
	"github.com/menawar/ecommerce-platform/services/order/internal/saga"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type Server struct {
	orderv1.UnimplementedOrderServiceServer
	q    *db.Queries
	saga *saga.Saga
	log  *slog.Logger
}

func NewServer(pool *pgxpool.Pool, sg *saga.Saga, log *slog.Logger) *Server {
	return &Server{q: db.New(pool), saga: sg, log: log}
}

// PlaceOrder runs the saga. The saga already returns gRPC status errors, so we
// pass them through.
func (s *Server) PlaceOrder(ctx context.Context, req *orderv1.PlaceOrderRequest) (*orderv1.PlaceOrderResponse, error) {
	res, err := s.saga.PlaceOrder(ctx, req.GetUserId(), req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	return &orderv1.PlaceOrderResponse{OrderId: res.OrderID, Status: string(res.Status)}, nil
}

func (s *Server) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	id, err := uuid.Parse(req.GetOrderId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "order_id must be a UUID")
	}
	o, err := s.q.GetOrder(ctx, pgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "order not found")
		}
		return nil, s.internal(ctx, "get order", err)
	}
	items, err := s.q.ListOrderItems(ctx, pgUUID(id))
	if err != nil {
		return nil, s.internal(ctx, "list order items", err)
	}
	return &orderv1.GetOrderResponse{Order: toProtoOrder(o, items)}, nil
}

func (s *Server) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	uid, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	page := req.GetPage()
	if page < 1 {
		page = 1
	}
	size := req.GetPageSize()
	if size < 1 {
		size = defaultPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}

	rows, err := s.q.ListOrdersByUser(ctx, db.ListOrdersByUserParams{
		UserID: pgUUID(uid),
		Limit:  size,
		Offset: (page - 1) * size,
	})
	if err != nil {
		return nil, s.internal(ctx, "list orders", err)
	}
	orders := make([]*orderv1.Order, 0, len(rows))
	for _, o := range rows {
		orders = append(orders, toProtoOrder(o, nil)) // summary: no items
	}
	return &orderv1.ListOrdersResponse{Orders: orders}, nil
}

func (s *Server) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	st, err := s.saga.Cancel(ctx, req.GetOrderId())
	if err != nil {
		return nil, err
	}
	return &orderv1.CancelOrderResponse{Status: string(st)}, nil
}

func (s *Server) internal(ctx context.Context, msg string, err error) error {
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

func toProtoOrder(o db.Order, items []db.OrderItem) *orderv1.Order {
	out := &orderv1.Order{
		Id:         uuidStr(o.ID),
		UserId:     uuidStr(o.UserID),
		Status:     o.Status,
		TotalCents: o.TotalCents,
		Currency:   o.Currency,
		PaymentId:  uuidStr(o.PaymentID),
	}
	if o.CreatedAt.Valid {
		out.CreatedAt = o.CreatedAt.Time.Unix()
	}
	for _, it := range items {
		out.Items = append(out.Items, &orderv1.OrderItem{
			ProductId:  uuidStr(it.ProductID),
			Name:       it.Name,
			PriceCents: it.PriceCents,
			Quantity:   it.Quantity,
		})
	}
	return out
}
