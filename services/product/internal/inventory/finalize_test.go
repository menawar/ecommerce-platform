package inventory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/menawar/ecommerce-platform/services/product/internal/inventory"
)

func TestRelease_ReturnsStock(t *testing.T) {
	pool := testPool(t)
	r := inventory.NewReserver(pool)
	productID := seedProduct(t, pool, 10)
	rid := uuid.NewString()

	if err := r.Reserve(context.Background(), rid, []inventory.Item{{ProductID: productID, Quantity: 4}}); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if got := reservedQty(t, pool, productID); got != 4 {
		t.Fatalf("reserved after reserve = %d, want 4", got)
	}

	if err := r.Release(context.Background(), rid); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if got := reservedQty(t, pool, productID); got != 0 {
		t.Errorf("reserved after release = %d, want 0", got)
	}

	// Idempotent: releasing again is a no-op.
	if err := r.Release(context.Background(), rid); err != nil {
		t.Errorf("idempotent Release: %v", err)
	}
	if got := reservedQty(t, pool, productID); got != 0 {
		t.Errorf("reserved after second release = %d, want 0", got)
	}
}

func TestCommit_DecrementsQuantity(t *testing.T) {
	pool := testPool(t)
	r := inventory.NewReserver(pool)
	productID := seedProduct(t, pool, 10)
	rid := uuid.NewString()

	if err := r.Reserve(context.Background(), rid, []inventory.Item{{ProductID: productID, Quantity: 3}}); err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if err := r.Commit(context.Background(), rid); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if q := totalQty(t, pool, productID); q != 7 {
		t.Errorf("quantity after commit = %d, want 7", q)
	}
	if got := reservedQty(t, pool, productID); got != 0 {
		t.Errorf("reserved after commit = %d, want 0", got)
	}

	// Idempotent re-commit.
	if err := r.Commit(context.Background(), rid); err != nil {
		t.Errorf("idempotent Commit: %v", err)
	}
	if q := totalQty(t, pool, productID); q != 7 {
		t.Errorf("quantity after second commit = %d, want 7", q)
	}
}

func TestRelease_AfterCommit_Conflict(t *testing.T) {
	pool := testPool(t)
	r := inventory.NewReserver(pool)
	productID := seedProduct(t, pool, 10)
	rid := uuid.NewString()

	_ = r.Reserve(context.Background(), rid, []inventory.Item{{ProductID: productID, Quantity: 2}})
	if err := r.Commit(context.Background(), rid); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := r.Release(context.Background(), rid); !errors.Is(err, inventory.ErrReservationConflict) {
		t.Errorf("Release after Commit: want ErrReservationConflict, got %v", err)
	}
}
