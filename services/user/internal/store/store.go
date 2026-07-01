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
	// ErrPasswordResetNotFound is returned when no password-reset token matches.
	ErrPasswordResetNotFound = errors.New("store: password reset token not found")
	// ErrAddressNotFound is returned when no address matches (or isn't the caller's).
	ErrAddressNotFound = errors.New("store: address not found")
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
	// UpdatePassword replaces the account's password hash (used by password reset).
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
	// DeleteAccount erases the account: it anonymises the users row (PII tombstoned)
	// and purges the user's addresses and tokens in one transaction. It is
	// idempotent — the returned bool is true only if THIS call performed the
	// erasure (false = already deleted), so the caller emits user.deleted once.
	DeleteAccount(ctx context.Context, userID string) (bool, error)
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

// PasswordResetToken is a single-use, short-lived token emailed as a link; using
// it lets the holder set a new password. Same shape as VerificationToken but a
// distinct type so the two flows can't be confused.
type PasswordResetToken struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
	UsedAt    *time.Time // nil = not yet used
}

// Usable reports whether the token can still reset a password: not used, not expired.
func (t PasswordResetToken) Usable(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}

// PasswordResetTokenStore persists password-reset tokens. A separate store (and
// type) from the verification and refresh stores for the same reason: so one
// Postgres struct can implement it without Save/Get method collisions.
//
// Unlike VerificationTokenStore.Use (best-effort), reset consumption is ATOMIC:
// Consume flips an unused token and reports whether THIS caller won, so the
// password update can be gated on it — closing the check-then-act race and the
// replay window that a password credential can't tolerate.
type PasswordResetTokenStore interface {
	Save(ctx context.Context, t PasswordResetToken) error
	Get(ctx context.Context, token string) (PasswordResetToken, error)
	// Consume marks the token used only if it was still unused, returning true iff
	// this call performed the flip (exactly one concurrent caller gets true).
	Consume(ctx context.Context, token string) (bool, error)
	// InvalidateForUser spends all of a user's outstanding (unused) reset tokens,
	// so issuing a new link revokes any prior one.
	InvalidateForUser(ctx context.Context, userID string) error
}

// Address is a saved shipping address in a user's address book. Orders snapshot a
// copy at checkout, so editing an address never rewrites past orders.
type Address struct {
	ID         string
	UserID     string
	Label      string
	Recipient  string
	Phone      string
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string
	IsDefault  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AddressStore persists the per-user address book. Every read/write is scoped by
// user_id so a caller can only ever touch their OWN addresses — passing someone
// else's id yields ErrAddressNotFound, not their data. A separate store/type from
// the others so its Create/Get don't collide on the shared Postgres struct.
type AddressStore interface {
	// Create inserts a and returns it with its generated id/timestamps. If
	// a.IsDefault, it atomically clears any prior default first.
	Create(ctx context.Context, a Address) (Address, error)
	ListByUser(ctx context.Context, userID string) ([]Address, error)
	Get(ctx context.Context, userID, id string) (Address, error)
	// Update replaces the mutable fields of the user's address (a.ID, a.UserID);
	// the default flag is managed only via SetDefault. ErrAddressNotFound if the
	// address isn't the caller's.
	Update(ctx context.Context, a Address) error
	Delete(ctx context.Context, userID, id string) error
	// SetDefault makes id the user's only default (clears others first, in one tx).
	SetDefault(ctx context.Context, userID, id string) error
}
