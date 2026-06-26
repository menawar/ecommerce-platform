package server_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/menawar/ecommerce-platform/pkg/postgres"
	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	"github.com/menawar/ecommerce-platform/services/product/internal/server"
)

// Integration tests: need a real, migrated productdb; SKIP otherwise. Because the
// server commits its own transactions, we isolate by TRUNCATE-ing the tables at
// the start of each test rather than the rollback trick (the test can't wrap the
// server's internal transaction).
func newTestClient(t *testing.T) productv1.ProductServiceClient {
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
	truncate(t, pool)

	srv := server.NewServer(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer()
	productv1.RegisterProductServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return productv1.NewProductServiceClient(conn)
}

func truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "TRUNCATE products, inventory, categories RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestCreateProduct_HappyPath(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	const imageURL = "https://cdn.example.com/widget.png"
	create, err := client.CreateProduct(ctx, &productv1.CreateProductRequest{
		Sku: "WIDGET-1", Name: "Widget", Description: "A widget",
		PriceCents: 1999, Currency: "NGN", InitialQuantity: 25, ImageUrl: imageURL,
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	p := create.GetProduct()
	if p.GetId() == "" || p.GetAvailable() != 25 || p.GetPriceCents() != 1999 || p.GetImageUrl() != imageURL {
		t.Fatalf("created product = %+v", p)
	}

	got, err := client.GetProduct(ctx, &productv1.GetProductRequest{ProductId: p.GetId()})
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	// image_url must survive the round-trip through the DB and the read path.
	if got.GetProduct().GetSku() != "WIDGET-1" || got.GetProduct().GetAvailable() != 25 || got.GetProduct().GetImageUrl() != imageURL {
		t.Errorf("got %+v", got.GetProduct())
	}
}

func TestCreateProduct_DuplicateSku(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	req := &productv1.CreateProductRequest{Sku: "DUP", Name: "x", PriceCents: 1, InitialQuantity: 1}

	if _, err := client.CreateProduct(ctx, req); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := client.CreateProduct(ctx, req); status.Code(err) != codes.AlreadyExists {
		t.Errorf("want AlreadyExists, got %v", err)
	}
}

func TestCreateProduct_Validation(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	cases := map[string]*productv1.CreateProductRequest{
		"empty sku":      {Sku: "", Name: "x", PriceCents: 1, InitialQuantity: 1},
		"negative price": {Sku: "p", Name: "x", PriceCents: -1, InitialQuantity: 1},
		"negative qty":   {Sku: "p", Name: "x", PriceCents: 1, InitialQuantity: -1},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := client.CreateProduct(ctx, req); status.Code(err) != codes.InvalidArgument {
				t.Errorf("want InvalidArgument, got %v", err)
			}
		})
	}
}

func TestGetProduct_NotFoundAndInvalid(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	if _, err := client.GetProduct(ctx, &productv1.GetProductRequest{ProductId: uuid.NewString()}); status.Code(err) != codes.NotFound {
		t.Errorf("want NotFound for random id, got %v", err)
	}
	if _, err := client.GetProduct(ctx, &productv1.GetProductRequest{ProductId: "not-a-uuid"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("want InvalidArgument for bad id, got %v", err)
	}
}

func TestListProducts_Pagination(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	for _, sku := range []string{"a", "b", "c"} {
		if _, err := client.CreateProduct(ctx, &productv1.CreateProductRequest{
			Sku: sku, Name: sku, PriceCents: 1, InitialQuantity: 1,
		}); err != nil {
			t.Fatalf("create %s: %v", sku, err)
		}
	}

	resp, err := client.ListProducts(ctx, &productv1.ListProductsRequest{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if len(resp.GetProducts()) != 2 {
		t.Errorf("page size = %d, want 2", len(resp.GetProducts()))
	}
	if resp.GetTotal() != 3 {
		t.Errorf("total = %d, want 3", resp.GetTotal())
	}
}

func TestListProducts_Sort(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	// Distinct prices so the ordering is deterministic regardless of insert order.
	for sku, price := range map[string]int64{"cheap": 100, "mid": 500, "pricey": 900} {
		if _, err := client.CreateProduct(ctx, &productv1.CreateProductRequest{
			Sku: sku, Name: sku, PriceCents: price, InitialQuantity: 1,
		}); err != nil {
			t.Fatalf("create %s: %v", sku, err)
		}
	}

	asc, err := client.ListProducts(ctx, &productv1.ListProductsRequest{Sort: "price_asc"})
	if err != nil {
		t.Fatalf("ListProducts price_asc: %v", err)
	}
	ap := asc.GetProducts()
	if len(ap) != 3 {
		t.Fatalf("price_asc returned %d products, want 3", len(ap))
	}
	for i := 1; i < len(ap); i++ {
		if ap[i-1].GetPriceCents() > ap[i].GetPriceCents() {
			t.Errorf("price_asc not ascending: %d before %d", ap[i-1].GetPriceCents(), ap[i].GetPriceCents())
		}
	}

	desc, err := client.ListProducts(ctx, &productv1.ListProductsRequest{Sort: "price_desc"})
	if err != nil {
		t.Fatalf("ListProducts price_desc: %v", err)
	}
	dp := desc.GetProducts()
	for i := 1; i < len(dp); i++ {
		if dp[i-1].GetPriceCents() < dp[i].GetPriceCents() {
			t.Errorf("price_desc not descending: %d before %d", dp[i-1].GetPriceCents(), dp[i].GetPriceCents())
		}
	}

	// An unknown sort key must fall back to the default ordering, never error.
	if _, err := client.ListProducts(ctx, &productv1.ListProductsRequest{Sort: "bogus"}); err != nil {
		t.Fatalf("unknown sort should not error: %v", err)
	}
}
