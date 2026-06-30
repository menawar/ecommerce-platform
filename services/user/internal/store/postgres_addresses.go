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

// PostgresAddresses is the production AddressStore. A separate type from Postgres
// (like the token stores) so its Create/Get don't collide with the user repo's.
type PostgresAddresses struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

var _ AddressStore = (*PostgresAddresses)(nil)

func NewPostgresAddresses(pool *pgxpool.Pool) *PostgresAddresses {
	return &PostgresAddresses{pool: pool, q: db.New(pool)}
}

func (p *PostgresAddresses) Create(ctx context.Context, a Address) (Address, error) {
	uid, err := uuid.Parse(a.UserID)
	if err != nil {
		return Address{}, fmt.Errorf("store: invalid user id %q: %w", a.UserID, err)
	}
	// A new default must atomically unseat the old one.
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return Address{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := p.q.WithTx(tx)

	if a.IsDefault {
		if err := q.ClearDefaultAddresses(ctx, pgtype.UUID{Bytes: uid, Valid: true}); err != nil {
			return Address{}, fmt.Errorf("store: clear default: %w", err)
		}
	}
	row, err := q.CreateAddress(ctx, db.CreateAddressParams{
		UserID:     pgtype.UUID{Bytes: uid, Valid: true},
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
	})
	if err != nil {
		return Address{}, fmt.Errorf("store: create address: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Address{}, err
	}
	return toAddress(row), nil
}

func (p *PostgresAddresses) ListByUser(ctx context.Context, userID string) ([]Address, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("store: invalid user id %q: %w", userID, err)
	}
	rows, err := p.q.ListAddressesByUser(ctx, pgtype.UUID{Bytes: uid, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("store: list addresses: %w", err)
	}
	out := make([]Address, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAddress(r))
	}
	return out, nil
}

func (p *PostgresAddresses) Get(ctx context.Context, userID, id string) (Address, error) {
	aid, err := uuid.Parse(id)
	if err != nil {
		return Address{}, ErrAddressNotFound
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return Address{}, ErrAddressNotFound
	}
	row, err := p.q.GetAddress(ctx, db.GetAddressParams{
		ID:     pgtype.UUID{Bytes: aid, Valid: true},
		UserID: pgtype.UUID{Bytes: uid, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Address{}, ErrAddressNotFound
		}
		return Address{}, fmt.Errorf("store: get address: %w", err)
	}
	return toAddress(row), nil
}

func (p *PostgresAddresses) Update(ctx context.Context, a Address) error {
	aid, err := uuid.Parse(a.ID)
	if err != nil {
		return ErrAddressNotFound
	}
	uid, err := uuid.Parse(a.UserID)
	if err != nil {
		return ErrAddressNotFound
	}
	rows, err := p.q.UpdateAddress(ctx, db.UpdateAddressParams{
		ID:         pgtype.UUID{Bytes: aid, Valid: true},
		UserID:     pgtype.UUID{Bytes: uid, Valid: true},
		Label:      a.Label,
		Recipient:  a.Recipient,
		Phone:      a.Phone,
		Line1:      a.Line1,
		Line2:      a.Line2,
		City:       a.City,
		State:      a.State,
		PostalCode: a.PostalCode,
		Country:    a.Country,
	})
	if err != nil {
		return fmt.Errorf("store: update address: %w", err)
	}
	if rows == 0 {
		return ErrAddressNotFound
	}
	return nil
}

func (p *PostgresAddresses) Delete(ctx context.Context, userID, id string) error {
	aid, err := uuid.Parse(id)
	if err != nil {
		return ErrAddressNotFound
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrAddressNotFound
	}
	rows, err := p.q.DeleteAddress(ctx, db.DeleteAddressParams{
		ID:     pgtype.UUID{Bytes: aid, Valid: true},
		UserID: pgtype.UUID{Bytes: uid, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("store: delete address: %w", err)
	}
	if rows == 0 {
		return ErrAddressNotFound
	}
	return nil
}

func (p *PostgresAddresses) SetDefault(ctx context.Context, userID, id string) error {
	aid, err := uuid.Parse(id)
	if err != nil {
		return ErrAddressNotFound
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrAddressNotFound
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := p.q.WithTx(tx)

	if err := q.ClearDefaultAddresses(ctx, pgtype.UUID{Bytes: uid, Valid: true}); err != nil {
		return fmt.Errorf("store: clear default: %w", err)
	}
	rows, err := q.SetAddressDefault(ctx, db.SetAddressDefaultParams{
		ID:     pgtype.UUID{Bytes: aid, Valid: true},
		UserID: pgtype.UUID{Bytes: uid, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("store: set default: %w", err)
	}
	if rows == 0 {
		return ErrAddressNotFound // not the caller's address — don't commit the clear
	}
	return tx.Commit(ctx)
}

func toAddress(r db.Address) Address {
	return Address{
		ID:         uuid.UUID(r.ID.Bytes).String(),
		UserID:     uuid.UUID(r.UserID.Bytes).String(),
		Label:      r.Label,
		Recipient:  r.Recipient,
		Phone:      r.Phone,
		Line1:      r.Line1,
		Line2:      r.Line2,
		City:       r.City,
		State:      r.State,
		PostalCode: r.PostalCode,
		Country:    r.Country,
		IsDefault:  r.IsDefault,
		CreatedAt:  r.CreatedAt.Time,
		UpdatedAt:  r.UpdatedAt.Time,
	}
}
