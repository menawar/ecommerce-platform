package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/menawar/ecommerce-platform/services/user/internal/db"
)

// PostgresVerificationTokens is the production VerificationTokenStore, backed by
// userdb via sqlc. It is a SEPARATE type from Postgres (rather than more methods
// on it) so its Save/Get don't collide with the refresh-token store's.
type PostgresVerificationTokens struct {
	q *db.Queries
}

var _ VerificationTokenStore = (*PostgresVerificationTokens)(nil)

func NewPostgresVerificationTokens(pool *pgxpool.Pool) *PostgresVerificationTokens {
	return &PostgresVerificationTokens{q: db.New(pool)}
}

func (p *PostgresVerificationTokens) Save(ctx context.Context, t VerificationToken) error {
	tok, err := uuid.Parse(t.Token)
	if err != nil {
		return fmt.Errorf("store: invalid token %q: %w", t.Token, err)
	}
	uid, err := uuid.Parse(t.UserID)
	if err != nil {
		return fmt.Errorf("store: invalid user id %q: %w", t.UserID, err)
	}
	return p.q.SaveVerificationToken(ctx, db.SaveVerificationTokenParams{
		Token:     pgtype.UUID{Bytes: tok, Valid: true},
		UserID:    pgtype.UUID{Bytes: uid, Valid: true},
		ExpiresAt: pgtype.Timestamptz{Time: t.ExpiresAt, Valid: true},
	})
}

func (p *PostgresVerificationTokens) Get(ctx context.Context, token string) (VerificationToken, error) {
	tok, err := uuid.Parse(token)
	if err != nil {
		return VerificationToken{}, ErrVerificationNotFound
	}
	row, err := p.q.GetVerificationToken(ctx, pgtype.UUID{Bytes: tok, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return VerificationToken{}, ErrVerificationNotFound
		}
		return VerificationToken{}, fmt.Errorf("store: get verification token: %w", err)
	}
	vt := VerificationToken{
		Token:     uuid.UUID(row.Token.Bytes).String(),
		UserID:    uuid.UUID(row.UserID.Bytes).String(),
		ExpiresAt: row.ExpiresAt.Time,
	}
	if row.UsedAt.Valid {
		used := row.UsedAt.Time
		vt.UsedAt = &used
	}
	return vt, nil
}

func (p *PostgresVerificationTokens) Use(ctx context.Context, token string) error {
	tok, err := uuid.Parse(token)
	if err != nil {
		return ErrVerificationNotFound
	}
	return p.q.UseVerificationToken(ctx, pgtype.UUID{Bytes: tok, Valid: true})
}
