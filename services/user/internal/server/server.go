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
	"net/url"
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

// verificationTTL is how long an email-verification link stays valid.
const verificationTTL = 24 * time.Hour

// Server implements userv1.UserServiceServer. It depends on INTERFACES
// (store.Repository, auth.TokenIssuer, auth.TokenValidator) so it can be tested
// with an in-memory store and swapped to Postgres without touching this code.
type Server struct {
	userv1.UnimplementedUserServiceServer

	repo               store.Repository
	refreshTokens      store.RefreshTokenStore
	verificationTokens store.VerificationTokenStore
	accessIssuer       auth.TokenIssuer    // 15m tokens
	refreshIssuer      auth.TokenIssuer    // 7d tokens
	validator          auth.TokenValidator // validates ACCESS tokens
	refreshValidator   auth.TokenValidator // validates REFRESH tokens (rejects access)
	publisher          events.Publisher    // emits user.* events; nil = don't publish
	verifyBaseURL      string              // base URL the verification link points at
	log                *slog.Logger

	// dummyHash is a real bcrypt hash we compare against when an email is not
	// found, so the "no such user" path costs the same ~60ms as a real wrong
	// password — closing the timing side-channel for account enumeration.
	dummyHash string
}

func NewServer(
	repo store.Repository,
	refreshTokens store.RefreshTokenStore,
	verificationTokens store.VerificationTokenStore,
	accessIssuer, refreshIssuer auth.TokenIssuer,
	validator, refreshValidator auth.TokenValidator,
	publisher events.Publisher,
	verifyBaseURL string,
	log *slog.Logger,
) *Server {
	dummy, _ := auth.HashPassword("timing-equalizer-not-a-real-password")
	return &Server{
		repo:               repo,
		refreshTokens:      refreshTokens,
		verificationTokens: verificationTokens,
		accessIssuer:       accessIssuer,
		refreshIssuer:      refreshIssuer,
		validator:          validator,
		refreshValidator:   refreshValidator,
		publisher:          publisher,
		verifyBaseURL:      verifyBaseURL,
		log:                log,
		dummyHash:          dummy,
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
	// Issue the email-verification token, also best-effort: a failure here must not
	// fail an otherwise-successful registration — the user can ask for a fresh link
	// via ResendVerification.
	if err := s.issueVerification(ctx, u.ID, u.Email); err != nil {
		s.log.ErrorContext(ctx, "issue verification on register", "err", err, "user_id", u.ID)
	}
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

	access, refresh, expiresAt, err := s.issueTokenPair(ctx, u.ID, u.Role)
	if err != nil {
		return nil, err
	}
	return &userv1.LoginResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    expiresAt.Unix(),
	}, nil
}

// issueTokenPair mints an access + refresh token and PERSISTS the refresh jti so
// it can be revoked/rotated later. Shared by Login and RefreshToken.
func (s *Server) issueTokenPair(ctx context.Context, userID, role string) (access, refresh string, accessExp time.Time, err error) {
	access, _, accessExp, err = s.accessIssuer.Issue(userID, role)
	if err != nil {
		return "", "", time.Time{}, s.internal(ctx, "issue access token", err)
	}
	refresh, jti, refreshExp, err := s.refreshIssuer.Issue(userID, role)
	if err != nil {
		return "", "", time.Time{}, s.internal(ctx, "issue refresh token", err)
	}
	if err := s.refreshTokens.Save(ctx, store.RefreshToken{JTI: jti, UserID: userID, ExpiresAt: refreshExp}); err != nil {
		return "", "", time.Time{}, s.internal(ctx, "save refresh token", err)
	}
	return access, refresh, accessExp, nil
}

// RefreshToken rotates a refresh token. It validates the token, checks it's still
// active in the store, then revokes it and issues a fresh pair. Presenting a token
// that's already been revoked (e.g. a stolen, already-rotated one) revokes the
// whole user's tokens — refresh-token reuse is the signal of theft.
func (s *Server) RefreshToken(ctx context.Context, req *userv1.RefreshTokenRequest) (*userv1.RefreshTokenResponse, error) {
	claims, err := s.refreshValidator.Validate(req.GetRefreshToken())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}
	rt, err := s.refreshTokens.Get(ctx, claims.TokenID)
	if err != nil {
		if errors.Is(err, store.ErrRefreshNotFound) {
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		}
		return nil, s.internal(ctx, "get refresh token", err)
	}
	if rt.RevokedAt != nil {
		// Reuse of a revoked token → likely theft. Nuke every session for the user.
		if err := s.refreshTokens.RevokeAllForUser(ctx, rt.UserID); err != nil {
			s.log.ErrorContext(ctx, "revoke-all on reuse", "err", err, "user_id", rt.UserID)
		}
		return nil, status.Error(codes.Unauthenticated, "refresh token reuse detected")
	}
	if !rt.Active(time.Now()) {
		return nil, status.Error(codes.Unauthenticated, "refresh token expired")
	}

	// Rotate: revoke the presented token, then mint + persist a new pair.
	if err := s.refreshTokens.Revoke(ctx, claims.TokenID); err != nil {
		return nil, s.internal(ctx, "revoke old refresh token", err)
	}
	access, refresh, accessExp, err := s.issueTokenPair(ctx, claims.UserID, claims.Role)
	if err != nil {
		return nil, err
	}
	return &userv1.RefreshTokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    accessExp.Unix(),
	}, nil
}

// Logout revokes the presented refresh token. It is idempotent and lenient: an
// invalid or already-revoked token still returns success — the caller wants the
// session gone regardless.
func (s *Server) Logout(ctx context.Context, req *userv1.LogoutRequest) (*userv1.LogoutResponse, error) {
	claims, err := s.refreshValidator.Validate(req.GetRefreshToken())
	if err != nil {
		return &userv1.LogoutResponse{}, nil // nothing to revoke; treat as logged out
	}
	if err := s.refreshTokens.Revoke(ctx, claims.TokenID); err != nil && !errors.Is(err, store.ErrRefreshNotFound) {
		return nil, s.internal(ctx, "revoke refresh token", err)
	}
	return &userv1.LogoutResponse{}, nil
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
		UserId:        u.ID,
		Email:         u.Email,
		FullName:      u.FullName,
		Role:          u.Role,
		EmailVerified: u.EmailVerified,
	}, nil
}

// VerifyEmail consumes a single-use verification token and marks the account's
// email verified. Re-clicking a link whose account is already verified returns
// success (idempotent); an unknown, expired, or already-spent token for an
// unverified account is reported as a single generic InvalidArgument so the
// response can't be used to probe which tokens exist.
func (s *Server) VerifyEmail(ctx context.Context, req *userv1.VerifyEmailRequest) (*userv1.VerifyEmailResponse, error) {
	if strings.TrimSpace(req.GetToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}
	vt, err := s.verificationTokens.Get(ctx, req.GetToken())
	if err != nil {
		if errors.Is(err, store.ErrVerificationNotFound) {
			return nil, errInvalidVerification()
		}
		return nil, s.internal(ctx, "get verification token", err)
	}

	if !vt.Usable(time.Now()) {
		// Expired or already used. If the account is already verified, a repeat
		// click is a no-op success rather than a confusing error.
		if u, gerr := s.repo.GetByID(ctx, vt.UserID); gerr == nil && u.EmailVerified {
			return &userv1.VerifyEmailResponse{}, nil
		}
		return nil, errInvalidVerification()
	}

	if err := s.repo.SetEmailVerified(ctx, vt.UserID); err != nil {
		return nil, s.internal(ctx, "set email verified", err)
	}
	if err := s.verificationTokens.Use(ctx, vt.Token); err != nil {
		// The flag is already flipped; failing to mark the token used only leaves it
		// reusable until expiry (still harmless — SetEmailVerified is idempotent).
		s.log.ErrorContext(ctx, "mark verification token used", "err", err, "user_id", vt.UserID)
	}
	s.log.InfoContext(ctx, "email verified", "user_id", vt.UserID)
	return &userv1.VerifyEmailResponse{}, nil
}

// ResendVerification issues a fresh verification token for the caller. If the
// account is already verified it is a no-op success — there is nothing to send.
func (s *Server) ResendVerification(ctx context.Context, req *userv1.ResendVerificationRequest) (*userv1.ResendVerificationResponse, error) {
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
	if u.EmailVerified {
		return &userv1.ResendVerificationResponse{}, nil
	}
	if err := s.issueVerification(ctx, u.ID, u.Email); err != nil {
		return nil, s.internal(ctx, "resend verification", err)
	}
	return &userv1.ResendVerificationResponse{}, nil
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

// issueVerification mints + persists a single-use verification token, logs the
// link (handy in dev), and best-effort emits user.verification_requested carrying
// the link so the Notification service can render/send the email. A persistence
// failure is returned so callers can decide whether it is fatal (resend) or not
// (register).
func (s *Server) issueVerification(ctx context.Context, userID, email string) error {
	token := uuid.NewString()
	if err := s.verificationTokens.Save(ctx, store.VerificationToken{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(verificationTTL),
	}); err != nil {
		return err
	}
	// The link carries a live, single-use credential — never log the URL itself.
	// The dev-visible link surfaces via the Notification service's LogSender; here
	// we log only the user_id for correlation.
	s.log.InfoContext(ctx, "email verification link issued", "user_id", userID)
	s.publishVerificationRequested(ctx, userID, email, s.verifyURL(token))
	return nil
}

// verifyURL builds the link a recipient clicks. The path matches the web app's
// verification page; the token rides as a query param.
func (s *Server) verifyURL(token string) string {
	base := strings.TrimRight(s.verifyBaseURL, "/")
	return base + "/verify-email?token=" + url.QueryEscape(token)
}

type verificationRequested struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	VerifyURL string `json:"verify_url"`
}

func (s *Server) publishVerificationRequested(ctx context.Context, userID, email, link string) {
	if s.publisher == nil {
		return
	}
	env, err := events.New("user.verification_requested", verificationRequested{UserID: userID, Email: email, VerifyURL: link})
	if err != nil {
		s.log.ErrorContext(ctx, "build user.verification_requested", "err", err)
		return
	}
	payload, err := env.Marshal()
	if err != nil {
		s.log.ErrorContext(ctx, "marshal user.verification_requested", "err", err)
		return
	}
	if err := s.publisher.Publish(ctx, "user.verification_requested", payload); err != nil {
		s.log.ErrorContext(ctx, "publish user.verification_requested", "err", err)
	}
}

func errInvalidCredentials() error {
	return status.Error(codes.Unauthenticated, "invalid email or password")
}

func errInvalidVerification() error {
	return status.Error(codes.InvalidArgument, "verification token is invalid or expired")
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
