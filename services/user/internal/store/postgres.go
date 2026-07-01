package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/menawar/ecommerce-platform/services/user/internal/db"
)

// Postgres is the production Repository, backed by userdb via sqlc. It implements
// the SAME store.Repository interface as Memory — which is the whole point of the
// Phase 1 design: swapping this in is a one-line wiring change in userd, and the
// gRPC handlers don't change at all.
type Postgres struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// Compile-time check that Postgres satisfies the interface.
var _ Repository = (*Postgres)(nil)

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool, q: db.New(pool)}
}

func (p *Postgres) Create(ctx context.Context, u User) error {
	id, err := uuid.Parse(u.ID)
	if err != nil {
		return fmt.Errorf("store: invalid user id %q: %w", u.ID, err)
	}

	err = p.q.CreateUser(ctx, db.CreateUserParams{
		ID:           pgtype.UUID{Bytes: id, Valid: true},
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		FullName:     u.FullName,
		Role:         u.Role,
		CreatedAt:    pgtype.Timestamptz{Time: u.CreatedAt, Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: u.UpdatedAt, Valid: true},
	})
	if err != nil {
		// The CITEXT UNIQUE constraint surfaces as SQLSTATE 23505 — the same
		// "email already registered" outcome the Memory store returns explicitly.
		if isUniqueViolation(err) {
			return ErrEmailTaken
		}
		return fmt.Errorf("store: create user: %w", err)
	}
	return nil
}

func (p *Postgres) GetByEmail(ctx context.Context, email string) (User, error) {
	// email is CITEXT, so the comparison is case-insensitive in the database — no
	// need to lowercase here as the Memory store did.
	row, err := p.q.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, mapGetErr(err)
	}
	return toUser(row), nil
}

func (p *Postgres) GetByID(ctx context.Context, id string) (User, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		// A malformed id can't match any row — treat as not found, not a fault.
		return User{}, ErrNotFound
	}
	row, err := p.q.GetUserByID(ctx, pgtype.UUID{Bytes: uid, Valid: true})
	if err != nil {
		return User{}, mapGetErr(err)
	}
	return toUser(row), nil
}

func mapGetErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("store: query user: %w", err)
}

func toUser(r db.User) User {
	return User{
		ID:            uuid.UUID(r.ID.Bytes).String(),
		Email:         r.Email,
		PasswordHash:  r.PasswordHash,
		FullName:      r.FullName,
		Role:          r.Role,
		EmailVerified: r.EmailVerified,
		CreatedAt:     r.CreatedAt.Time,
		UpdatedAt:     r.UpdatedAt.Time,
	}
}

func (p *Postgres) SetEmailVerified(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("store: invalid user id %q: %w", userID, err)
	}
	return p.q.SetEmailVerified(ctx, pgtype.UUID{Bytes: uid, Valid: true})
}

func (p *Postgres) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("store: invalid user id %q: %w", userID, err)
	}
	return p.q.UpdatePassword(ctx, db.UpdatePasswordParams{
		ID:           pgtype.UUID{Bytes: uid, Valid: true},
		PasswordHash: passwordHash,
	})
}

func (p *Postgres) DeleteAccount(ctx context.Context, userID string) (bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, fmt.Errorf("store: invalid user id %q: %w", userID, err)
	}
	pgID := pgtype.UUID{Bytes: uid, Valid: true}

	// One transaction: tombstone the users row and purge everything keyed to the
	// user, so an account is never left half-erased.
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := p.q.WithTx(tx)

	rows, err := q.AnonymizeUser(ctx, pgID)
	if err != nil {
		return false, fmt.Errorf("store: anonymize user: %w", err)
	}
	if rows == 0 {
		return false, nil // already deleted — idempotent no-op, no event
	}
	for _, purge := range []func(context.Context, pgtype.UUID) error{
		q.DeleteUserAddresses,
		q.DeleteUserRefreshTokens,
		q.DeleteUserVerificationTokens,
		q.DeleteUserPasswordResetTokens,
	} {
		if err := purge(ctx, pgID); err != nil {
			return false, fmt.Errorf("store: purge user data: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// --- RefreshTokenStore ---

var _ RefreshTokenStore = (*Postgres)(nil)

func (p *Postgres) Save(ctx context.Context, t RefreshToken) error {
	jti, err := uuid.Parse(t.JTI)
	if err != nil {
		return fmt.Errorf("store: invalid jti %q: %w", t.JTI, err)
	}
	uid, err := uuid.Parse(t.UserID)
	if err != nil {
		return fmt.Errorf("store: invalid user id %q: %w", t.UserID, err)
	}
	return p.q.SaveRefreshToken(ctx, db.SaveRefreshTokenParams{
		Jti:       pgtype.UUID{Bytes: jti, Valid: true},
		UserID:    pgtype.UUID{Bytes: uid, Valid: true},
		ExpiresAt: pgtype.Timestamptz{Time: t.ExpiresAt, Valid: true},
	})
}

func (p *Postgres) Get(ctx context.Context, jti string) (RefreshToken, error) {
	id, err := uuid.Parse(jti)
	if err != nil {
		return RefreshToken{}, ErrRefreshNotFound
	}
	row, err := p.q.GetRefreshToken(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RefreshToken{}, ErrRefreshNotFound
		}
		return RefreshToken{}, fmt.Errorf("store: get refresh token: %w", err)
	}
	rt := RefreshToken{
		JTI:       uuid.UUID(row.Jti.Bytes).String(),
		UserID:    uuid.UUID(row.UserID.Bytes).String(),
		ExpiresAt: row.ExpiresAt.Time,
	}
	if row.RevokedAt.Valid {
		revoked := row.RevokedAt.Time
		rt.RevokedAt = &revoked
	}
	return rt, nil
}

func (p *Postgres) Revoke(ctx context.Context, jti string) error {
	id, err := uuid.Parse(jti)
	if err != nil {
		return ErrRefreshNotFound
	}
	return p.q.RevokeRefreshToken(ctx, pgtype.UUID{Bytes: id, Valid: true})
}

func (p *Postgres) RevokeAllForUser(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("store: invalid user id %q: %w", userID, err)
	}
	return p.q.RevokeAllUserRefreshTokens(ctx, pgtype.UUID{Bytes: uid, Valid: true})
}
