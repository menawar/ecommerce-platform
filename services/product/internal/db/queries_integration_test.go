package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/menawar/ecommerce-platform/pkg/postgres"
	"github.com/menawar/ecommerce-platform/services/product/internal/db"
)

// These are INTEGRATION tests: they need a real, migrated productdb. They SKIP
// (not fail) when the database is unreachable, so `go test ./...` stays green on
// a machine without infra. To run them: `make infra-up && make product-migrate-up`.
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
	return pool
}

// queriesInTx returns a Queries bound to a transaction that is ALWAYS rolled back
// at test end. This is the isolation trick for DB tests: each test runs in its own
// transaction and discards it, so tests never persist data, never see each other's
// writes, and need no manual cleanup. db.New accepts the tx because pgx.Tx
// satisfies sqlc's DBTX interface.
func queriesInTx(t *testing.T, pool *pgxpool.Pool) *db.Queries {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return db.New(tx)
}

func TestCreateAndGetProduct(t *testing.T) {
	ctx := context.Background()
	q := queriesInTx(t, testPool(t))

	cat, err := q.CreateCategory(ctx, db.CreateCategoryParams{Name: "Electronics", Slug: "electronics"})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	created, err := q.CreateProduct(ctx, db.CreateProductParams{
		Sku: "SKU-1", Name: "Widget", Description: "A widget",
		PriceCents: 1999, Currency: "NGN", CategoryID: cat.ID,
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	got, err := q.GetProduct(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if got.Sku != "SKU-1" || got.PriceCents != 1999 || got.Name != "Widget" {
		t.Errorf("got %+v", got)
	}

	// A missing id returns pgx.ErrNoRows — the caller maps this to NotFound.
	_, err = q.GetProduct(ctx, pgtype.UUID{}) // zero UUID, won't exist
	if err == nil {
		t.Error("GetProduct(zero uuid) should error")
	}
}

func TestListAndCount_FilterByCategory(t *testing.T) {
	ctx := context.Background()
	q := queriesInTx(t, testPool(t))

	catA, _ := q.CreateCategory(ctx, db.CreateCategoryParams{Name: "A", Slug: "a"})
	catB, _ := q.CreateCategory(ctx, db.CreateCategoryParams{Name: "B", Slug: "b"})
	mustStockedProduct(t, q, "a1", catA.ID)
	mustStockedProduct(t, q, "a2", catA.ID)
	mustStockedProduct(t, q, "b1", catB.ID)

	listA, err := q.ListProductsWithInventory(ctx, db.ListProductsWithInventoryParams{CategoryID: catA.ID, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("ListProductsWithInventory: %v", err)
	}
	if len(listA) != 2 {
		t.Errorf("category A list = %d, want 2", len(listA))
	}

	countA, _ := q.CountProducts(ctx, db.CountProductsParams{CategoryID: catA.ID})
	if countA != 2 {
		t.Errorf("category A count = %d, want 2", countA)
	}

	// NULL category filter -> all rows visible in this tx (the 3 we inserted).
	all, _ := q.ListProductsWithInventory(ctx, db.ListProductsWithInventoryParams{CategoryID: pgtype.UUID{}, Limit: 10, Offset: 0})
	if len(all) < 3 {
		t.Errorf("unfiltered list = %d, want >= 3", len(all))
	}

	// Search by name (ILIKE, case-insensitive) -> only "a1".
	found, err := q.ListProductsWithInventory(ctx, db.ListProductsWithInventoryParams{Search: strptr("A1"), Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("search list: %v", err)
	}
	if len(found) != 1 || found[0].Sku != "a1" {
		t.Errorf("search 'A1' = %d rows, want 1 (a1)", len(found))
	}
}

func TestInventoryRoundTrip(t *testing.T) {
	ctx := context.Background()
	q := queriesInTx(t, testPool(t))

	cat, _ := q.CreateCategory(ctx, db.CreateCategoryParams{Name: "Inv", Slug: "inv"})
	p := mustProduct(t, q, "inv-1", cat.ID)

	inv, err := q.CreateInventory(ctx, db.CreateInventoryParams{ProductID: p.ID, Quantity: 50})
	if err != nil {
		t.Fatalf("CreateInventory: %v", err)
	}
	if inv.Quantity != 50 || inv.Reserved != 0 || inv.Version != 0 {
		t.Errorf("new inventory = %+v, want qty 50 reserved 0 version 0", inv)
	}

	got, err := q.GetInventory(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetInventory: %v", err)
	}
	if got.Quantity != 50 {
		t.Errorf("GetInventory quantity = %d, want 50", got.Quantity)
	}
}

func mustProduct(t *testing.T, q *db.Queries, sku string, catID pgtype.UUID) db.Product {
	t.Helper()
	p, err := q.CreateProduct(context.Background(), db.CreateProductParams{
		Sku: sku, Name: sku, Description: "", PriceCents: 100, Currency: "NGN", CategoryID: catID,
	})
	if err != nil {
		t.Fatalf("CreateProduct(%s): %v", sku, err)
	}
	return p
}

// mustStockedProduct creates a product AND its inventory row, so it appears in the
// inventory-joined list/count queries.
func mustStockedProduct(t *testing.T, q *db.Queries, sku string, catID pgtype.UUID) db.Product {
	t.Helper()
	p := mustProduct(t, q, sku, catID)
	if _, err := q.CreateInventory(context.Background(), db.CreateInventoryParams{ProductID: p.ID, Quantity: 1}); err != nil {
		t.Fatalf("CreateInventory(%s): %v", sku, err)
	}
	return p
}

func strptr(s string) *string { return &s }
