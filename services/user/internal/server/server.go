// Package server implements the user.v1.UserService gRPC server. It is the
// composition point: it wires the persistence port (store.Repository), the
// password primitives (pkg/auth), and the token issuer/validator together, and
// translates domain outcomes into gRPC status codes.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/menawar/ecommerce-platform/pkg/auth"
	"github.com/menawar/ecommerce-platform/pkg/events"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

const minPasswordLen = 8

// Server implements userv1.UserServiceServer. It depends on INTERFACES
// (store.Repository, auth.TokenIssuer, auth.TokenValidator) so it can be tested
// with an in-memory store and swapped to Postgres without touching this code.
type Server struct {
	userv1.UnimplementedUserServiceServer

	repo          store.Repository
	accessIssuer  auth.TokenIssuer // 15m tokens
	refreshIssuer auth.TokenIssuer // 7d tokens
	validator     auth.TokenValidator
	publisher     events.Publisher // emits user.registered; nil = don't publish
	log           *slog.Logger

	// dummyHash is a real bcrypt hash we compare against when an email is not
	// found, so the "no such user" path costs the same ~60ms as a real wrong
	// password — closing the timing side-channel for account enumeration.
	dummyHash string
}

func NewServer(
	repo store.Repository,
	accessIssuer, refreshIssuer auth.TokenIssuer,
	validator auth.TokenValidator,
	publisher events.Publisher,
	log *slog.Logger,
) *Server {
	dummy, _ := auth.HashPassword("timing-equalizer-not-a-real-password")
	return &Server{
		repo:          repo,
		accessIssuer:  accessIssuer,
		refreshIssuer: refreshIssuer,
		validator:     validator,
		publisher:     publisher,
		log:           log,
		dummyHash:     dummy,
	}
}

// Register creates an account. Validation failures map to InvalidArgument, a
// taken email to AlreadyExists, anything unexpected to Internal (with the real
// cause logged, never returned).
func (s *Server) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterResponse, error) {
	email, err := normalizeEmail(req.GetEmail())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid email address")
	}
	if len(req.GetPassword()) < minPasswordLen {
		return nil, status.Errorf(codes.InvalidArgument, "password must be at least %d characters", minPasswordLen)
	}
	if strings.TrimSpace(req.GetFullName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "full_name is required")
	}

	hash, err := auth.HashPassword(req.GetPassword())
	if err != nil {
		return nil, s.internal(ctx, "hash password", err)
	}

	now := time.Now()
	u := store.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: hash,
		FullName:     req.GetFullName(),
		Role:         "customer", // self-registration is always a customer
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		if errors.Is(err, store.ErrEmailTaken) {
			return nil, status.Error(codes.AlreadyExists, "email already registered")
		}
		return nil, s.internal(ctx, "create user", err)
	}

	s.log.InfoContext(ctx, "registered user", "user_id", u.ID)
	// Emit user.registered (best-effort). Unlike the order saga's transactional
	// outbox, a welcome notification isn't worth holding the registration tx open
	// for — if the publish fails, the user is still registered; we just log it.
	s.publishUserRegistered(ctx, u.ID, u.Email)
	return &userv1.RegisterResponse{UserId: u.ID}, nil
}

// Login verifies credentials and issues tokens. CRUCIAL: a missing email and a
// wrong password return the IDENTICAL error (Unauthenticated, same message) and
// take the same time — otherwise the response leaks which emails are registered.
func (s *Server) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginResponse, error) {
	u, err := s.repo.GetByEmail(ctx, req.GetEmail())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Spend the same CPU as a real verify, then return the generic error.
			_ = auth.VerifyPassword(s.dummyHash, req.GetPassword())
			return nil, errInvalidCredentials()
		}
		return nil, s.internal(ctx, "lookup user", err)
	}

	if err := auth.VerifyPassword(u.PasswordHash, req.GetPassword()); err != nil {
		if errors.Is(err, auth.ErrPasswordMismatch) {
			return nil, errInvalidCredentials()
		}
		return nil, s.internal(ctx, "verify password", err)
	}

	access, expiresAt, err := s.accessIssuer.Issue(u.ID, u.Role)
	if err != nil {
		return nil, s.internal(ctx, "issue access token", err)
	}
	refresh, _, err := s.refreshIssuer.Issue(u.ID, u.Role)
	if err != nil {
		return nil, s.internal(ctx, "issue refresh token", err)
	}

	return &userv1.LoginResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    expiresAt.Unix(),
	}, nil
}

// ValidateToken reports whether a token is valid and, if so, who it belongs to.
// A bad/expired token is a normal answer (valid=false), NOT an RPC error — the
// Gateway asks "is this valid?" and always wants a boolean back.
func (s *Server) ValidateToken(ctx context.Context, req *userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
	claims, err := s.validator.Validate(req.GetToken())
	if err != nil {
		return &userv1.ValidateTokenResponse{Valid: false}, nil
	}
	return &userv1.ValidateTokenResponse{
		Valid:  true,
		UserId: claims.UserID,
		Role:   claims.Role,
	}, nil
}

// GetUser returns a user's profile by id for server-to-server callers. It never
// includes the password hash.
func (s *Server) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	if _, err := uuid.Parse(req.GetUserId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "user_id must be a UUID")
	}
	u, err := s.repo.GetByID(ctx, req.GetUserId())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, s.internal(ctx, "get user", err)
	}
	return &userv1.GetUserResponse{
		UserId:   u.ID,
		Email:    u.Email,
		FullName: u.FullName,
		Role:     u.Role,
	}, nil
}

type userRegistered struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

func (s *Server) publishUserRegistered(ctx context.Context, userID, email string) {
	if s.publisher == nil {
		return
	}
	env, err := events.New("user.registered", userRegistered{UserID: userID, Email: email})
	if err != nil {
		s.log.ErrorContext(ctx, "build user.registered", "err", err)
		return
	}
	payload, err := env.Marshal()
	if err != nil {
		s.log.ErrorContext(ctx, "marshal user.registered", "err", err)
		return
	}
	if err := s.publisher.Publish(ctx, "user.registered", payload); err != nil {
		s.log.ErrorContext(ctx, "publish user.registered", "err", err)
	}
}

func errInvalidCredentials() error {
	return status.Error(codes.Unauthenticated, "invalid email or password")
}

// internal logs the real cause and returns a leak-free Internal status. Keeping
// this in one place means no handler accidentally returns err.Error() to a client.
func (s *Server) internal(ctx context.Context, msg string, err error) error {
	s.log.ErrorContext(ctx, msg, "err", err)
	return status.Error(codes.Internal, "internal error")
}

func normalizeEmail(raw string) (string, error) {
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return "", err
	}
	return strings.ToLower(addr.Address), nil
}
