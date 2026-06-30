// Package store defines how the User service persists accounts, plus an
// in-memory implementation for Phase 1. The Postgres implementation arrives in
// Phase 2; because every caller depends on the Repository INTERFACE rather than
// a concrete type, swapping the implementation is a one-line change at wiring
// time and needs zero changes to the gRPC handlers.
package store

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrEmailTaken is returned by Create when the email already exists. It is a
	// normal business outcome (maps to AlreadyExists / HTTP 409), not a fault.
	ErrEmailTaken = errors.New("store: email already registered")
	// ErrNotFound is returned by lookups when no account matches.
	ErrNotFound = errors.New("store: user not found")
	// ErrRefreshNotFound is returned when no refresh token matches the jti.
	ErrRefreshNotFound = errors.New("store: refresh token not found")
	// ErrVerificationNotFound is returned when no verification token matches.
	ErrVerificationNotFound = errors.New("store: verification token not found")
)

// User is a persisted account. Fields mirror the userdb.users schema from the
// spec so the Phase 2 Postgres implementation maps 1:1.
type User struct {
	ID            string
	Email         string
	PasswordHash  string
	FullName      string
	Role          string
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Repository is the persistence PORT the service depends on. Every method takes
// context.Context first so the Postgres implementation can honor per-request
// deadlines and cancellation; the in-memory implementation accepts it and
// ignores it, keeping the signature identical so the swap is invisible to
// callers. This is "accept interfaces" — the service is written against this,
// never against a concrete store.
type Repository interface {
	Create(ctx context.Context, u User) error
	GetByEmail(ctx context.Context, email string) (User, error)
	GetByID(ctx context.Context, id string) (User, error)
	// SetEmailVerified flips email_verified to true for the account. It is
	// idempotent: verifying an already-verified account is a no-op success.
	SetEmailVerified(ctx context.Context, userID string) error
}

// RefreshToken is a persisted, revocable refresh-token record. We store only the
// jti (the token's id) — never the token itself — so a DB leak can't be replayed.
type RefreshToken struct {
	JTI       string
	UserID    string
	ExpiresAt time.Time
	RevokedAt *time.Time // nil = still active
}

// Active reports whether the token can still be used: not revoked, not expired.
func (t RefreshToken) Active(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}

// RefreshTokenStore tracks refresh tokens so sessions are revocable (logout) and
// rotatable (each use issues a new one and revokes the old).
type RefreshTokenStore interface {
	Save(ctx context.Context, t RefreshToken) error
	Get(ctx context.Context, jti string) (RefreshToken, error)
	Revoke(ctx context.Context, jti string) error
	RevokeAllForUser(ctx context.Context, userID string) error
}

// VerificationToken is a single-use, expiring token emailed to a new account as a
// link. Verifying flips the user's email_verified flag and marks the token used.
type VerificationToken struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
	UsedAt    *time.Time // nil = not yet used
}

// Usable reports whether the token can still verify an account: not already used
// and not expired.
func (t VerificationToken) Usable(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}

// VerificationTokenStore persists email-verification tokens. It is a SEPARATE
// store (not folded into Repository or RefreshTokenStore) so the concrete
// Postgres type can satisfy it without its Save/Get colliding with the
// refresh-token store's identically-named methods.
type VerificationTokenStore interface {
	Save(ctx context.Context, t VerificationToken) error
	Get(ctx context.Context, token string) (VerificationToken, error)
	// Use marks the token consumed. It is BEST-EFFORT: it only flips an unused
	// token (so a concurrent double-verify consumes it at most once) and does NOT
	// report whether the token existed — a missing or already-used token is a
	// silent no-op, not an error. Callers detect validity via Get + Usable first.
	Use(ctx context.Context, token string) error
}
