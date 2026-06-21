package inventory

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/menawar/ecommerce-platform/services/product/internal/db"
)

// Reservation status values, mirroring the CHECK constraint in the schema.
const (
	statusReserved  = "reserved"
	statusReleased  = "released"
	statusCommitted = "committed"
)

// ErrReservationConflict means the reservation is in a terminal state
// incompatible with the requested transition (e.g. releasing one already
// committed). It signals a saga bug, not a normal retry.
var ErrReservationConflict = errors.New("inventory: reservation in a conflicting state")

// Release is the saga's COMPENSATING action: it returns held stock to available.
// Idempotent (releasing an already-released reservation is a no-op).
func (r *Reserver) Release(ctx context.Context, reservationID string) error {
	return r.finalize(ctx, reservationID, statusReleased)
}

// Commit FINALIZES a reservation: the reserved units actually leave inventory
// (both quantity and reserved drop). Idempotent.
func (r *Reserver) Commit(ctx context.Context, reservationID string) error {
	return r.finalize(ctx, reservationID, statusCommitted)
}

// finalize transitions a reservation to target (released or committed) and
// applies the matching inventory change to every item — all in one transaction,
// with the reservation row locked FOR UPDATE so concurrent/replayed calls
// serialize and stay idempotent.
func (r *Reserver) finalize(ctx context.Context, reservationID, target string) error {
	rid, err := parseUUID(reservationID)
	if err != nil {
		return fmt.Errorf("inventory: invalid reservation id: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("inventory: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.q.WithTx(tx)

	status, err := q.GetReservationStatusForUpdate(ctx, rid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // unknown reservation: nothing to undo/finalize
		}
		return fmt.Errorf("inventory: lock reservation: %w", err)
	}

	switch status {
	case target:
		return nil // already in the target state -> idempotent no-op
	case statusReserved:
		// the only state we can transition FROM
	default:
		// e.g. Release after Commit, or Commit after Release.
		return ErrReservationConflict
	}

	items, err := q.ListReservationItems(ctx, rid)
	if err != nil {
		return fmt.Errorf("inventory: list reservation items: %w", err)
	}
	for _, item := range items {
		switch target {
		case statusReleased:
			err = q.ReleaseInventory(ctx, db.ReleaseInventoryParams{Quantity: item.Quantity, ProductID: item.ProductID})
		case statusCommitted:
			err = q.CommitInventory(ctx, db.CommitInventoryParams{Quantity: item.Quantity, ProductID: item.ProductID})
		}
		if err != nil {
			return fmt.Errorf("inventory: apply %s: %w", target, err)
		}
	}

	if err := q.SetReservationStatus(ctx, db.SetReservationStatusParams{ID: rid, Status: target}); err != nil {
		return fmt.Errorf("inventory: set status: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("inventory: commit: %w", err)
	}
	return nil
}
