package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

// Integration tests for the Postgres address store. Need a migrated userdb
// (migration 000005); SKIP otherwise.
func TestPostgres_Addresses(t *testing.T) {
	ctx := context.Background()
	pool := userPool(t)
	if _, err := pool.Exec(ctx, "TRUNCATE addresses"); err != nil {
		t.Fatalf("truncate addresses: %v", err)
	}
	as := store.NewPostgresAddresses(pool)
	u1, u2 := uuid.NewString(), uuid.NewString()

	a1, err := as.Create(ctx, store.Address{
		UserID: u1, Recipient: "Ada", Phone: "0803", Line1: "1 Rayfield",
		City: "Jos", State: "Plateau", Country: "NG",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	a2, err := as.Create(ctx, store.Address{
		UserID: u1, Recipient: "Ada", Phone: "0803", Line1: "2 Rayfield",
		City: "Jos", State: "Plateau", Country: "NG", IsDefault: true,
	})
	if err != nil {
		t.Fatalf("Create default: %v", err)
	}

	t.Run("list is default-first and user-scoped", func(t *testing.T) {
		list, err := as.ListByUser(ctx, u1)
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		if len(list) != 2 || list[0].ID != a2.ID || !list[0].IsDefault {
			t.Errorf("want a2 (default) first; got %+v", list)
		}
	})

	t.Run("ownership: other user can't read/update/delete", func(t *testing.T) {
		if _, err := as.Get(ctx, u2, a1.ID); !errors.Is(err, store.ErrAddressNotFound) {
			t.Errorf("Get(other): want ErrAddressNotFound, got %v", err)
		}
		bad := a1
		bad.UserID = u2
		if err := as.Update(ctx, bad); !errors.Is(err, store.ErrAddressNotFound) {
			t.Errorf("Update(other): want ErrAddressNotFound, got %v", err)
		}
		if err := as.Delete(ctx, u2, a1.ID); !errors.Is(err, store.ErrAddressNotFound) {
			t.Errorf("Delete(other): want ErrAddressNotFound, got %v", err)
		}
	})

	t.Run("SetDefault is exclusive", func(t *testing.T) {
		if err := as.SetDefault(ctx, u1, a1.ID); err != nil {
			t.Fatalf("SetDefault: %v", err)
		}
		g1, _ := as.Get(ctx, u1, a1.ID)
		g2, _ := as.Get(ctx, u1, a2.ID)
		if !g1.IsDefault || g2.IsDefault {
			t.Errorf("default should be exactly a1: a1.default=%v a2.default=%v", g1.IsDefault, g2.IsDefault)
		}
	})

	t.Run("update then delete", func(t *testing.T) {
		upd := a1
		upd.City = "Bukuru"
		if err := as.Update(ctx, upd); err != nil {
			t.Fatalf("Update: %v", err)
		}
		if got, _ := as.Get(ctx, u1, a1.ID); got.City != "Bukuru" {
			t.Errorf("city = %q, want Bukuru", got.City)
		}
		if err := as.Delete(ctx, u1, a1.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := as.Get(ctx, u1, a1.ID); !errors.Is(err, store.ErrAddressNotFound) {
			t.Errorf("after delete: want ErrAddressNotFound, got %v", err)
		}
	})
}
