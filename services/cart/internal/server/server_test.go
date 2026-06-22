package server_test

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	cartv1 "github.com/menawar/ecommerce-platform/proto/cart/v1"
	"github.com/menawar/ecommerce-platform/services/cart/internal/server"
	"github.com/menawar/ecommerce-platform/services/cart/internal/store"
)

func newClient(t *testing.T) cartv1.CartServiceClient {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	srv := server.NewServer(store.NewRedis(rc), slog.New(slog.NewTextHandler(io.Discard, nil)))
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer()
	cartv1.RegisterCartServiceServer(gs, srv)
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
	return cartv1.NewCartServiceClient(conn)
}

func TestCartLifecycle(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	user := uuid.NewString()
	p1, p2 := uuid.NewString(), uuid.NewString()

	// Add p1 x2, p2 x1.
	if _, err := c.AddItem(ctx, &cartv1.AddItemRequest{UserId: user, ProductId: p1, Quantity: 2}); err != nil {
		t.Fatalf("AddItem p1: %v", err)
	}
	addResp, err := c.AddItem(ctx, &cartv1.AddItemRequest{UserId: user, ProductId: p2, Quantity: 1})
	if err != nil {
		t.Fatalf("AddItem p2: %v", err)
	}
	if len(addResp.GetCart().GetItems()) != 2 {
		t.Fatalf("cart has %d items, want 2", len(addResp.GetCart().GetItems()))
	}

	// Update p1 to absolute 5.
	upd, _ := c.UpdateItem(ctx, &cartv1.UpdateItemRequest{UserId: user, ProductId: p1, Quantity: 5})
	if qtyOf(upd.GetCart(), p1) != 5 {
		t.Errorf("p1 qty after update = %d, want 5", qtyOf(upd.GetCart(), p1))
	}

	// Update p2 to 0 -> removed.
	upd2, _ := c.UpdateItem(ctx, &cartv1.UpdateItemRequest{UserId: user, ProductId: p2, Quantity: 0})
	if qtyOf(upd2.GetCart(), p2) != 0 || len(upd2.GetCart().GetItems()) != 1 {
		t.Errorf("after update p2->0: %+v", upd2.GetCart().GetItems())
	}

	// Remove p1 -> empty.
	rem, _ := c.RemoveItem(ctx, &cartv1.RemoveItemRequest{UserId: user, ProductId: p1})
	if len(rem.GetCart().GetItems()) != 0 {
		t.Errorf("after remove p1: %+v", rem.GetCart().GetItems())
	}

	// GetCart on empty -> empty.
	get, _ := c.GetCart(ctx, &cartv1.GetCartRequest{UserId: user})
	if len(get.GetCart().GetItems()) != 0 {
		t.Errorf("empty cart: %+v", get.GetCart().GetItems())
	}
}

func TestClearCart(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	user := uuid.NewString()
	_, _ = c.AddItem(ctx, &cartv1.AddItemRequest{UserId: user, ProductId: uuid.NewString(), Quantity: 1})

	if _, err := c.ClearCart(ctx, &cartv1.ClearCartRequest{UserId: user}); err != nil {
		t.Fatalf("ClearCart: %v", err)
	}
	get, _ := c.GetCart(ctx, &cartv1.GetCartRequest{UserId: user})
	if len(get.GetCart().GetItems()) != 0 {
		t.Errorf("cart not cleared: %+v", get.GetCart().GetItems())
	}
}

func TestValidation(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	user := uuid.NewString()

	cases := []struct {
		name string
		call func() error
	}{
		{"bad user_id", func() error {
			_, err := c.GetCart(ctx, &cartv1.GetCartRequest{UserId: "not-a-uuid"})
			return err
		}},
		{"bad product_id", func() error {
			_, err := c.AddItem(ctx, &cartv1.AddItemRequest{UserId: user, ProductId: "nope", Quantity: 1})
			return err
		}},
		{"non-positive add quantity", func() error {
			_, err := c.AddItem(ctx, &cartv1.AddItemRequest{UserId: user, ProductId: uuid.NewString(), Quantity: 0})
			return err
		}},
		{"negative update quantity", func() error {
			_, err := c.UpdateItem(ctx, &cartv1.UpdateItemRequest{UserId: user, ProductId: uuid.NewString(), Quantity: -1})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status.Code(tc.call()) != codes.InvalidArgument {
				t.Errorf("want InvalidArgument")
			}
		})
	}
}

func qtyOf(cart *cartv1.Cart, productID string) int32 {
	for _, it := range cart.GetItems() {
		if it.GetProductId() == productID {
			return it.GetQuantity()
		}
	}
	return 0
}
