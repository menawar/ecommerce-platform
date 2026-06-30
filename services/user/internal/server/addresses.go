package server

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

// maxAddressesPerUser caps the address book so an authenticated caller can't bloat
// the shared users DB by creating addresses without bound.
const maxAddressesPerUser = 50

// CreateAddress adds an address to the caller's book. user_id is supplied by the
// Gateway from the validated token, so the caller can only write their own book.
func (s *Server) CreateAddress(ctx context.Context, req *userv1.CreateAddressRequest) (*userv1.CreateAddressResponse, error) {
	if _, err := uuid.Parse(req.GetUserId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	in, err := validateAddressInput(req.GetAddress())
	if err != nil {
		return nil, err
	}
	in.UserID = req.GetUserId()
	in.IsDefault = req.GetIsDefault()

	existing, err := s.addresses.ListByUser(ctx, req.GetUserId())
	if err != nil {
		return nil, s.internal(ctx, "count addresses", err)
	}
	if len(existing) >= maxAddressesPerUser {
		return nil, status.Errorf(codes.FailedPrecondition, "address book is full (max %d)", maxAddressesPerUser)
	}
	// The first address a user saves becomes their default, so checkout always has
	// one to pre-select even if they didn't tick the box.
	if len(existing) == 0 {
		in.IsDefault = true
	}

	created, err := s.addresses.Create(ctx, in)
	if err != nil {
		return nil, s.internal(ctx, "create address", err)
	}
	return &userv1.CreateAddressResponse{Address: toProtoAddress(created)}, nil
}

func (s *Server) ListAddresses(ctx context.Context, req *userv1.ListAddressesRequest) (*userv1.ListAddressesResponse, error) {
	if _, err := uuid.Parse(req.GetUserId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	list, err := s.addresses.ListByUser(ctx, req.GetUserId())
	if err != nil {
		return nil, s.internal(ctx, "list addresses", err)
	}
	out := make([]*userv1.Address, 0, len(list))
	for _, a := range list {
		out = append(out, toProtoAddress(a))
	}
	return &userv1.ListAddressesResponse{Addresses: out}, nil
}

// GetAddress returns one of the caller's addresses by id (scoped by user_id). It
// is used server-to-server by the Order saga to snapshot the chosen address.
func (s *Server) GetAddress(ctx context.Context, req *userv1.GetAddressRequest) (*userv1.GetAddressResponse, error) {
	if _, err := uuid.Parse(req.GetUserId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	if _, err := uuid.Parse(req.GetAddressId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "address_id must be a UUID")
	}
	a, err := s.addresses.Get(ctx, req.GetUserId(), req.GetAddressId())
	if err != nil {
		if errors.Is(err, store.ErrAddressNotFound) {
			return nil, status.Error(codes.NotFound, "address not found")
		}
		return nil, s.internal(ctx, "get address", err)
	}
	return &userv1.GetAddressResponse{Address: toProtoAddress(a)}, nil
}

func (s *Server) UpdateAddress(ctx context.Context, req *userv1.UpdateAddressRequest) (*userv1.UpdateAddressResponse, error) {
	if _, err := uuid.Parse(req.GetUserId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	if _, err := uuid.Parse(req.GetAddressId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "address_id must be a UUID")
	}
	in, err := validateAddressInput(req.GetAddress())
	if err != nil {
		return nil, err
	}
	in.ID = req.GetAddressId()
	in.UserID = req.GetUserId()

	if err := s.addresses.Update(ctx, in); err != nil {
		if errors.Is(err, store.ErrAddressNotFound) {
			return nil, status.Error(codes.NotFound, "address not found")
		}
		return nil, s.internal(ctx, "update address", err)
	}
	return &userv1.UpdateAddressResponse{}, nil
}

func (s *Server) DeleteAddress(ctx context.Context, req *userv1.DeleteAddressRequest) (*userv1.DeleteAddressResponse, error) {
	if _, err := uuid.Parse(req.GetUserId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	if _, err := uuid.Parse(req.GetAddressId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "address_id must be a UUID")
	}
	if err := s.addresses.Delete(ctx, req.GetUserId(), req.GetAddressId()); err != nil {
		if errors.Is(err, store.ErrAddressNotFound) {
			return nil, status.Error(codes.NotFound, "address not found")
		}
		return nil, s.internal(ctx, "delete address", err)
	}
	return &userv1.DeleteAddressResponse{}, nil
}

func (s *Server) SetDefaultAddress(ctx context.Context, req *userv1.SetDefaultAddressRequest) (*userv1.SetDefaultAddressResponse, error) {
	if _, err := uuid.Parse(req.GetUserId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	if _, err := uuid.Parse(req.GetAddressId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "address_id must be a UUID")
	}
	if err := s.addresses.SetDefault(ctx, req.GetUserId(), req.GetAddressId()); err != nil {
		if errors.Is(err, store.ErrAddressNotFound) {
			return nil, status.Error(codes.NotFound, "address not found")
		}
		return nil, s.internal(ctx, "set default address", err)
	}
	return &userv1.SetDefaultAddressResponse{}, nil
}

// maxAddrFieldLen bounds each address field so a single row can't be enormous.
const maxAddrFieldLen = 256

// validateAddressInput checks the required fields and length caps, returning a
// store.Address with the mutable fields filled (id/user/default set by the caller).
// Country defaults to NG when omitted. Fields are checked in a STABLE order so the
// error message is deterministic (a map's random iteration order would flake it).
func validateAddressInput(in *userv1.AddressInput) (store.Address, error) {
	if in == nil {
		return store.Address{}, status.Error(codes.InvalidArgument, "address is required")
	}
	a := store.Address{
		Label:      strings.TrimSpace(in.GetLabel()),
		Recipient:  strings.TrimSpace(in.GetRecipient()),
		Phone:      strings.TrimSpace(in.GetPhone()),
		Line1:      strings.TrimSpace(in.GetLine1()),
		Line2:      strings.TrimSpace(in.GetLine2()),
		City:       strings.TrimSpace(in.GetCity()),
		State:      strings.TrimSpace(in.GetState()),
		PostalCode: strings.TrimSpace(in.GetPostalCode()),
		Country:    strings.TrimSpace(in.GetCountry()),
	}
	fields := []struct {
		name     string
		val      string
		required bool
	}{
		{"label", a.Label, false},
		{"recipient", a.Recipient, true},
		{"phone", a.Phone, true},
		{"line1", a.Line1, true},
		{"line2", a.Line2, false},
		{"city", a.City, true},
		{"state", a.State, true},
		{"postal_code", a.PostalCode, false},
		{"country", a.Country, false},
	}
	for _, f := range fields {
		if f.required && f.val == "" {
			return store.Address{}, status.Errorf(codes.InvalidArgument, "%s is required", f.name)
		}
		if len(f.val) > maxAddrFieldLen {
			return store.Address{}, status.Errorf(codes.InvalidArgument, "%s is too long (max %d characters)", f.name, maxAddrFieldLen)
		}
	}
	if a.Country == "" {
		a.Country = "NG"
	}
	return a, nil
}

func toProtoAddress(a store.Address) *userv1.Address {
	return &userv1.Address{
		Id:         a.ID,
		UserId:     a.UserID,
		Label:      a.Label,
		Recipient:  a.Recipient,
		Phone:      a.Phone,
		Line1:      a.Line1,
		Line2:      a.Line2,
		City:       a.City,
		State:      a.State,
		PostalCode: a.PostalCode,
		Country:    a.Country,
		IsDefault:  a.IsDefault,
		CreatedAt:  a.CreatedAt.Unix(),
	}
}
