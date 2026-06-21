package inventory_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/menawar/ecommerce-platform/pkg/postgres"
	"github.com/menawar/ecommerce-platform/services/product/internal/db"
	"github.com/menawar/ecommerce-platform/services/product/internal/inventory"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("PRODUCT_DB_URL")
	if url == "" {
		url = "postgres://ecommerce:ecommerce@localhost:5433/productdb?sslmode=disable"
	}
	pool, err := postgres.NewPool(context.Background(), url)
	if err != nil {
		t.Skipf("skipping integration test (productdb unavailable; run `make infra-up && make product-migrate-up`): %v", err)
	}
	t.Cleanup(pool.Close)
	_, err = pool.Exec(context.Background(),
		"TRUNCATE products, inventory, categories, stock_reservations, reservation_items RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

// seedProduct inserts a product with the given starting stock and returns its id.
func seedProduct(t *testing.T, pool *pgxpool.Pool, quantity int32) string {
	t.Helper()
	q := db.New(pool)
	ctx := context.Background()
	p, err := q.CreateProduct(ctx, db.CreateProductParams{
		Sku: uuid.NewString(), Name: "p", PriceCents: 100, Currency: "NGN", CategoryID: pgtype.UUID{},
	})
	if err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if _, err := q.CreateInventory(ctx, db.CreateInventoryParams{ProductID: p.ID, Quantity: quantity}); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}
	return uuid.UUID(p.ID.Bytes).String()
}

func reservedQty(t *testing.T, pool *pgxpool.Pool, productID string) int32 {
	t.Helper()
	pid, _ := uuid.Parse(productID)
	inv, err := db.New(pool).GetInventory(context.Background(), pgtype.UUID{Bytes: pid, Valid: true})
	if err != nil {
		t.Fatalf("get inventory: %v", err)
	}
	return inv.Reserved
}

func TestReserve_HappyPath(t *testing.T) {
	pool := testPool(t)
	r := inventory.NewReserver(pool)
	productID := seedProduct(t, pool, 10)

	err := r.Reserve(context.Background(), uuid.NewString(), []inventory.Item{{ProductID: productID, Quantity: 3}})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if got := reservedQty(t, pool, productID); got != 3 {
		t.Errorf("reserved = %d, want 3", got)
	}
}

func TestReserve_Insufficient(t *testing.T) {
	pool := testPool(t)
	r := inventory.NewReserver(pool)
	productID := seedProduct(t, pool, 5)

	err := r.Reserve(context.Background(), uuid.NewString(), []inventory.Item{{ProductID: productID, Quantity: 10}})
	if !errors.Is(err, inventory.ErrInsufficientStock) {
		t.Fatalf("want ErrInsufficientStock, got %v", err)
	}
	if got := reservedQty(t, pool, productID); got != 0 {
		t.Errorf("reserved = %d, want 0 (nothing reserved on failure)", got)
	}
}

// TestReserve_Idempotent proves replaying the same reservation_id does NOT reserve
// twice — the saga can safely retry.
func TestReserve_Idempotent(t *testing.T) {
	pool := testPool(t)
	r := inventory.NewReserver(pool)
	productID := seedProduct(t, pool, 10)
	rid := uuid.NewString()
	items := []inventory.Item{{ProductID: productID, Quantity: 3}}

	if err := r.Reserve(context.Background(), rid, items); err != nil {
		t.Fatalf("first Reserve: %v", err)
	}
	if err := r.Reserve(context.Background(), rid, items); err != nil {
		t.Fatalf("replay Reserve: %v", err)
	}
	if got := reservedQty(t, pool, productID); got != 3 {
		t.Errorf("reserved = %d after replay, want 3 (not 6)", got)
	}
}

// TestReserve_MultiItemAllOrNothing proves atomicity across items: if the 2nd item
// is short, the 1st item's reserve is rolled back too.
func TestReserve_MultiItemAllOrNothing(t *testing.T) {
	pool := testPool(t)
	r := inventory.NewReserver(pool)
	plenty := seedProduct(t, pool, 10)
	scarce := seedProduct(t, pool, 1)

	err := r.Reserve(context.Background(), uuid.NewString(), []inventory.Item{
		{ProductID: plenty, Quantity: 5}, // would succeed alone...
		{ProductID: scarce, Quantity: 5}, // ...but this fails, so neither persists
	})
	if !errors.Is(err, inventory.ErrInsufficientStock) {
		t.Fatalf("want ErrInsufficientStock, got %v", err)
	}
	if got := reservedQty(t, pool, plenty); got != 0 {
		t.Errorf("plenty reserved = %d, want 0 (rolled back with the failing item)", got)
	}
}

// TestReserve_ConcurrentNeverOversells is the mandatory race test: fire many
// goroutines at limited stock and assert it never oversells. Run with -race.
func TestReserve_ConcurrentNeverOversells(t *testing.T) {
	pool := testPool(t)
	r := inventory.NewReserver(pool)

	const stock = 10
	const goroutines = 30 // 3x the stock: most must be rejected
	productID := seedProduct(t, pool, stock)

	var succeeded, insufficient int64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			// Distinct reservation id per goroutine, so this tests stock contention,
			// not idempotency.
			err := r.Reserve(context.Background(), uuid.NewString(),
				[]inventory.Item{{ProductID: productID, Quantity: 1}})
			switch {
			case err == nil:
				atomic.AddInt64(&succeeded, 1)
			case errors.Is(err, inventory.ErrInsufficientStock):
				atomic.AddInt64(&insufficient, 1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if succeeded != stock {
		t.Errorf("succeeded = %d, want exactly %d", succeeded, stock)
	}
	if insufficient != goroutines-stock {
		t.Errorf("insufficient = %d, want %d", insufficient, goroutines-stock)
	}
	if got := reservedQty(t, pool, productID); got != stock {
		t.Errorf("final reserved = %d, want %d — OVERSOLD if higher", got, stock)
	}
}
