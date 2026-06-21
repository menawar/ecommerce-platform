package server

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	"github.com/menawar/ecommerce-platform/services/product/internal/inventory"
)

// ReserveStock holds stock for the saga. Insufficient stock is reported as
// success=false (a normal outcome), NOT a gRPC error — the order saga branches on
// it to compensate. Genuine faults (bad input, DB error) are gRPC errors.
func (s *Server) ReserveStock(ctx context.Context, req *productv1.ReserveStockRequest) (*productv1.ReserveStockResponse, error) {
	if _, err := uuid.Parse(req.GetReservationId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "reservation_id must be a UUID")
	}
	if len(req.GetItems()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "items must not be empty")
	}

	items := make([]inventory.Item, 0, len(req.GetItems()))
	for _, it := range req.GetItems() {
		if _, err := uuid.Parse(it.GetProductId()); err != nil {
			return nil, status.Error(codes.InvalidArgument, "item product_id must be a UUID")
		}
		if it.GetQuantity() <= 0 {
			return nil, status.Error(codes.InvalidArgument, "item quantity must be positive")
		}
		items = append(items, inventory.Item{ProductID: it.GetProductId(), Quantity: it.GetQuantity()})
	}

	err := s.reserver.Reserve(ctx, req.GetReservationId(), items)
	switch {
	case err == nil:
		return &productv1.ReserveStockResponse{Success: true}, nil
	case errors.Is(err, inventory.ErrInsufficientStock):
		return &productv1.ReserveStockResponse{Success: false}, nil
	default:
		return nil, s.internal(ctx, "reserve stock", err)
	}
}

func (s *Server) ReleaseStock(ctx context.Context, req *productv1.ReleaseStockRequest) (*productv1.ReleaseStockResponse, error) {
	if _, err := uuid.Parse(req.GetReservationId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "reservation_id must be a UUID")
	}
	if err := s.reserver.Release(ctx, req.GetReservationId()); err != nil {
		if errors.Is(err, inventory.ErrReservationConflict) {
			return nil, status.Error(codes.FailedPrecondition, "reservation cannot be released in its current state")
		}
		return nil, s.internal(ctx, "release stock", err)
	}
	return &productv1.ReleaseStockResponse{}, nil
}

func (s *Server) CommitStock(ctx context.Context, req *productv1.CommitStockRequest) (*productv1.CommitStockResponse, error) {
	if _, err := uuid.Parse(req.GetReservationId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "reservation_id must be a UUID")
	}
	if err := s.reserver.Commit(ctx, req.GetReservationId()); err != nil {
		if errors.Is(err, inventory.ErrReservationConflict) {
			return nil, status.Error(codes.FailedPrecondition, "reservation cannot be committed in its current state")
		}
		return nil, s.internal(ctx, "commit stock", err)
	}
	return &productv1.CommitStockResponse{}, nil
}
