package server

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
	"github.com/menawar/ecommerce-platform/services/order/internal/db"
)

const maxShippingFieldLen = 256

// ListShippingMethods returns shipping options. active_only=true is the checkout
// view (only sellable options); false is the admin view (includes disabled ones).
func (s *Server) ListShippingMethods(ctx context.Context, req *orderv1.ListShippingMethodsRequest) (*orderv1.ListShippingMethodsResponse, error) {
	var rows []db.ShippingMethod
	var err error
	if req.GetActiveOnly() {
		rows, err = s.q.ListActiveShippingMethods(ctx)
	} else {
		rows, err = s.q.ListShippingMethods(ctx)
	}
	if err != nil {
		return nil, s.internal(ctx, "list shipping methods", err)
	}
	out := make([]*orderv1.ShippingMethod, 0, len(rows))
	for _, r := range rows {
		out = append(out, toProtoShippingMethod(r))
	}
	return &orderv1.ListShippingMethodsResponse{Methods: out}, nil
}

func (s *Server) CreateShippingMethod(ctx context.Context, req *orderv1.CreateShippingMethodRequest) (*orderv1.CreateShippingMethodResponse, error) {
	in, err := validateShippingInput(req.GetMethod())
	if err != nil {
		return nil, err
	}
	row, err := s.q.CreateShippingMethod(ctx, db.CreateShippingMethodParams{
		Name:        in.GetName(),
		Description: strings.TrimSpace(in.GetDescription()),
		PriceCents:  in.GetPriceCents(),
		SortOrder:   in.GetSortOrder(),
		Active:      in.GetActive(),
	})
	if err != nil {
		return nil, s.internal(ctx, "create shipping method", err)
	}
	return &orderv1.CreateShippingMethodResponse{Method: toProtoShippingMethod(row)}, nil
}

func (s *Server) UpdateShippingMethod(ctx context.Context, req *orderv1.UpdateShippingMethodRequest) (*orderv1.UpdateShippingMethodResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "id must be a UUID")
	}
	in, err := validateShippingInput(req.GetMethod())
	if err != nil {
		return nil, err
	}
	row, err := s.q.UpdateShippingMethod(ctx, db.UpdateShippingMethodParams{
		ID:          pgUUID(id),
		Name:        in.GetName(),
		Description: strings.TrimSpace(in.GetDescription()),
		PriceCents:  in.GetPriceCents(),
		SortOrder:   in.GetSortOrder(),
		Active:      in.GetActive(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "shipping method not found")
		}
		return nil, s.internal(ctx, "update shipping method", err)
	}
	return &orderv1.UpdateShippingMethodResponse{Method: toProtoShippingMethod(row)}, nil
}

func (s *Server) DeleteShippingMethod(ctx context.Context, req *orderv1.DeleteShippingMethodRequest) (*orderv1.DeleteShippingMethodResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "id must be a UUID")
	}
	rows, err := s.q.DeleteShippingMethod(ctx, pgUUID(id))
	if err != nil {
		return nil, s.internal(ctx, "delete shipping method", err)
	}
	if rows == 0 {
		return nil, status.Error(codes.NotFound, "shipping method not found")
	}
	return &orderv1.DeleteShippingMethodResponse{}, nil
}

// validateShippingInput trims+checks the mutable fields. price_cents>=0 is also a
// DB CHECK, but rejecting here gives a clean InvalidArgument instead of Internal.
func validateShippingInput(in *orderv1.ShippingMethodInput) (*orderv1.ShippingMethodInput, error) {
	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "method is required")
	}
	name := strings.TrimSpace(in.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if len(name) > maxShippingFieldLen || len(in.GetDescription()) > maxShippingFieldLen {
		return nil, status.Errorf(codes.InvalidArgument, "name/description too long (max %d characters)", maxShippingFieldLen)
	}
	if in.GetPriceCents() < 0 {
		return nil, status.Error(codes.InvalidArgument, "price_cents must be >= 0")
	}
	in.Name = name
	return in, nil
}

func toProtoShippingMethod(m db.ShippingMethod) *orderv1.ShippingMethod {
	return &orderv1.ShippingMethod{
		Id:          uuid.UUID(m.ID.Bytes).String(),
		Name:        m.Name,
		Description: m.Description,
		PriceCents:  m.PriceCents,
		SortOrder:   m.SortOrder,
		Active:      m.Active,
	}
}
