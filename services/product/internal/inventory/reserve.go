// Package inventory holds stock-mutation logic that must be transactional and
// idempotent — the operations the order saga depends on. Reserve is the first;
// Release and Commit follow in a later step.
package inventory

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/menawar/ecommerce-platform/services/product/internal/db"
)

// ErrInsufficientStock means at least one item couldn't be fully reserved. It is
// a normal business outcome (the saga compensates), not a server fault.
var ErrInsufficientStock = errors.New("inventory: insufficient stock")

// Item is one line of a reservation.
type Item struct {
	ProductID string
	Quantity  int32
}

// Reserver performs stock reservations against the pool.
type Reserver struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewReserver(pool *pgxpool.Pool) *Reserver {
	return &Reserver{pool: pool, q: db.New(pool)}
}

// Reserve holds stock for every item under reservationID, atomically and
// idempotently:
//
//   - ATOMIC: all items reserve or none do. The whole thing runs in one
//     transaction; if any item lacks stock we return ErrInsufficientStock and the
//     deferred Rollback undoes the reserves already made in this tx.
//   - IDEMPOTENT: replaying the same reservationID is a no-op returning nil. The
//     reservation row's primary key is the gate — a duplicate insert is a unique
//     violation we read as "already reserved".
//   - OVERSELL-PROOF: each item uses a conditional UPDATE that only succeeds when
//     enough stock is available; the row lock serializes concurrent reservers.
func (r *Reserver) Reserve(ctx context.Context, reservationID string, items []Item) error {
	rid, err := parseUUID(reservationID)
	if err != nil {
		return fmt.Errorf("inventory: invalid reservation id: %w", err)
	}
	if len(items) == 0 {
		return fmt.Errorf("inventory: no items to reserve")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("inventory: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful Commit
	q := r.q.WithTx(tx)

	// Idempotency gate. Inserting the reservation row first means a replay (or a
	// concurrent duplicate) hits the primary-key unique violation, which we treat
	// as "already reserved" -> success. The PK row lock also serializes concurrent
	// same-id requests so only one ever does the real reserving.
	if err := q.InsertReservation(ctx, rid); err != nil {
		if isUniqueViolation(err) {
			return nil // idempotent replay
		}
		return fmt.Errorf("inventory: insert reservation: %w", err)
	}

	for _, item := range items {
		pid, err := parseUUID(item.ProductID)
		if err != nil {
			return fmt.Errorf("inventory: invalid product id %q: %w", item.ProductID, err)
		}

		affected, err := q.ReserveInventory(ctx, db.ReserveInventoryParams{
			Quantity:  item.Quantity,
			ProductID: pid,
		})
		if err != nil {
			return fmt.Errorf("inventory: reserve update: %w", err)
		}
		if affected == 0 {
			// The conditional UPDATE matched no row: not enough available stock.
			// Returning triggers the deferred Rollback, undoing every reserve made
			// so far in this tx -> all-or-nothing across items.
			return ErrInsufficientStock
		}

		if err := q.InsertReservationItem(ctx, db.InsertReservationItemParams{
			ReservationID: rid,
			ProductID:     pid,
			Quantity:      item.Quantity,
		}); err != nil {
			return fmt.Errorf("inventory: insert reservation item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("inventory: commit: %w", err)
	}
	return nil
}

// --- local pg helpers (small; mirror server's — candidate for a shared pgutil) ---

func parseUUID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
