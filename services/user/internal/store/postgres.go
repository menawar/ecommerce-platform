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
	q *db.Queries
}

// Compile-time check that Postgres satisfies the interface.
var _ Repository = (*Postgres)(nil)

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{q: db.New(pool)}
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
		ID:           uuid.UUID(r.ID.Bytes).String(),
		Email:        r.Email,
		PasswordHash: r.PasswordHash,
		FullName:     r.FullName,
		Role:         r.Role,
		CreatedAt:    r.CreatedAt.Time,
		UpdatedAt:    r.UpdatedAt.Time,
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
