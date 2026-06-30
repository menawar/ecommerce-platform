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

// PostgresPasswordResetTokens is the production PasswordResetTokenStore, backed by
// userdb via sqlc. A separate type from Postgres (like the verification store) so
// its Save/Get don't collide with the other token stores' methods.
type PostgresPasswordResetTokens struct {
	q *db.Queries
}

var _ PasswordResetTokenStore = (*PostgresPasswordResetTokens)(nil)

func NewPostgresPasswordResetTokens(pool *pgxpool.Pool) *PostgresPasswordResetTokens {
	return &PostgresPasswordResetTokens{q: db.New(pool)}
}

func (p *PostgresPasswordResetTokens) Save(ctx context.Context, t PasswordResetToken) error {
	tok, err := uuid.Parse(t.Token)
	if err != nil {
		return fmt.Errorf("store: invalid token %q: %w", t.Token, err)
	}
	uid, err := uuid.Parse(t.UserID)
	if err != nil {
		return fmt.Errorf("store: invalid user id %q: %w", t.UserID, err)
	}
	return p.q.SavePasswordResetToken(ctx, db.SavePasswordResetTokenParams{
		Token:     pgtype.UUID{Bytes: tok, Valid: true},
		UserID:    pgtype.UUID{Bytes: uid, Valid: true},
		ExpiresAt: pgtype.Timestamptz{Time: t.ExpiresAt, Valid: true},
	})
}

func (p *PostgresPasswordResetTokens) Get(ctx context.Context, token string) (PasswordResetToken, error) {
	tok, err := uuid.Parse(token)
	if err != nil {
		return PasswordResetToken{}, ErrPasswordResetNotFound
	}
	row, err := p.q.GetPasswordResetToken(ctx, pgtype.UUID{Bytes: tok, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PasswordResetToken{}, ErrPasswordResetNotFound
		}
		return PasswordResetToken{}, fmt.Errorf("store: get password reset token: %w", err)
	}
	prt := PasswordResetToken{
		Token:     uuid.UUID(row.Token.Bytes).String(),
		UserID:    uuid.UUID(row.UserID.Bytes).String(),
		ExpiresAt: row.ExpiresAt.Time,
	}
	if row.UsedAt.Valid {
		used := row.UsedAt.Time
		prt.UsedAt = &used
	}
	return prt, nil
}

func (p *PostgresPasswordResetTokens) Consume(ctx context.Context, token string) (bool, error) {
	tok, err := uuid.Parse(token)
	if err != nil {
		return false, nil // not a real token → nothing to consume, nobody won
	}
	rows, err := p.q.ConsumePasswordResetToken(ctx, pgtype.UUID{Bytes: tok, Valid: true})
	if err != nil {
		return false, fmt.Errorf("store: consume password reset token: %w", err)
	}
	return rows == 1, nil
}

func (p *PostgresPasswordResetTokens) InvalidateForUser(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("store: invalid user id %q: %w", userID, err)
	}
	return p.q.InvalidateUserPasswordResetTokens(ctx, pgtype.UUID{Bytes: uid, Valid: true})
}
