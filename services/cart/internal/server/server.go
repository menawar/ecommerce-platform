// Package server implements the cart.v1.CartService gRPC server over the Store
// interface. It validates input, maps store.Item to proto, and translates errors
// to status codes. It deliberately does NOT check that a product exists or has
// stock — the cart records intentions; existence/stock/price are settled at
// checkout by the Order service.
package server

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cartv1 "github.com/menawar/ecommerce-platform/proto/cart/v1"
	"github.com/menawar/ecommerce-platform/services/cart/internal/store"
)

type Server struct {
	cartv1.UnimplementedCartServiceServer
	store store.Store
	log   *slog.Logger
}

func NewServer(s store.Store, log *slog.Logger) *Server {
	return &Server{store: s, log: log}
}

func (s *Server) GetCart(ctx context.Context, req *cartv1.GetCartRequest) (*cartv1.GetCartResponse, error) {
	if err := validateUUID(req.GetUserId(), "user_id"); err != nil {
		return nil, err
	}
	items, err := s.store.Get(ctx, req.GetUserId())
	if err != nil {
		return nil, s.internal(ctx, "get cart", err)
	}
	return &cartv1.GetCartResponse{Cart: toCart(req.GetUserId(), items)}, nil
}

func (s *Server) AddItem(ctx context.Context, req *cartv1.AddItemRequest) (*cartv1.AddItemResponse, error) {
	if err := validateUUID(req.GetUserId(), "user_id"); err != nil {
		return nil, err
	}
	if err := validateUUID(req.GetProductId(), "product_id"); err != nil {
		return nil, err
	}
	if req.GetQuantity() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "quantity must be positive")
	}
	items, err := s.store.AddItem(ctx, req.GetUserId(), req.GetProductId(), req.GetQuantity())
	if err != nil {
		return nil, s.internal(ctx, "add item", err)
	}
	return &cartv1.AddItemResponse{Cart: toCart(req.GetUserId(), items)}, nil
}

func (s *Server) UpdateItem(ctx context.Context, req *cartv1.UpdateItemRequest) (*cartv1.UpdateItemResponse, error) {
	if err := validateUUID(req.GetUserId(), "user_id"); err != nil {
		return nil, err
	}
	if err := validateUUID(req.GetProductId(), "product_id"); err != nil {
		return nil, err
	}
	// quantity 0 is allowed here — it means "remove this line". Negative is invalid.
	if req.GetQuantity() < 0 {
		return nil, status.Error(codes.InvalidArgument, "quantity must not be negative")
	}
	items, err := s.store.SetItem(ctx, req.GetUserId(), req.GetProductId(), req.GetQuantity())
	if err != nil {
		return nil, s.internal(ctx, "update item", err)
	}
	return &cartv1.UpdateItemResponse{Cart: toCart(req.GetUserId(), items)}, nil
}

func (s *Server) RemoveItem(ctx context.Context, req *cartv1.RemoveItemRequest) (*cartv1.RemoveItemResponse, error) {
	if err := validateUUID(req.GetUserId(), "user_id"); err != nil {
		return nil, err
	}
	if err := validateUUID(req.GetProductId(), "product_id"); err != nil {
		return nil, err
	}
	items, err := s.store.RemoveItem(ctx, req.GetUserId(), req.GetProductId())
	if err != nil {
		return nil, s.internal(ctx, "remove item", err)
	}
	return &cartv1.RemoveItemResponse{Cart: toCart(req.GetUserId(), items)}, nil
}

func (s *Server) ClearCart(ctx context.Context, req *cartv1.ClearCartRequest) (*cartv1.ClearCartResponse, error) {
	if err := validateUUID(req.GetUserId(), "user_id"); err != nil {
		return nil, err
	}
	if err := s.store.Clear(ctx, req.GetUserId()); err != nil {
		return nil, s.internal(ctx, "clear cart", err)
	}
	return &cartv1.ClearCartResponse{}, nil
}

func (s *Server) internal(ctx context.Context, msg string, err error) error {
	s.log.ErrorContext(ctx, msg, "err", err)
	return status.Error(codes.Internal, "internal error")
}

func validateUUID(v, field string) error {
	if _, err := uuid.Parse(v); err != nil {
		return status.Errorf(codes.InvalidArgument, "%s must be a UUID", field)
	}
	return nil
}

func toCart(userID string, items []store.Item) *cartv1.Cart {
	out := make([]*cartv1.CartItem, 0, len(items))
	for _, it := range items {
		out = append(out, &cartv1.CartItem{ProductId: it.ProductID, Quantity: it.Quantity})
	}
	return &cartv1.Cart{UserId: userID, Items: out}
}
