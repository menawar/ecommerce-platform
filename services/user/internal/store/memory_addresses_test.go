package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

var _ store.AddressStore = (*store.MemoryAddresses)(nil)

func sampleAddr(userID string, isDefault bool) store.Address {
	return store.Address{
		UserID: userID, Recipient: "Ada Lovelace", Phone: "08030000000",
		Line1: "1 Rayfield Rd", City: "Jos", State: "Plateau", Country: "NG",
		IsDefault: isDefault,
	}
}

func TestMemoryAddresses_CreateAndList(t *testing.T) {
	ctx := context.Background()
	as := store.NewMemoryAddresses()

	a1, err := as.Create(ctx, sampleAddr("u1", false))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a1.ID == "" {
		t.Fatal("Create should assign an id")
	}
	a2, _ := as.Create(ctx, sampleAddr("u1", true)) // default, newer
	_, _ = as.Create(ctx, sampleAddr("u2", false))  // other user

	list, err := as.ListByUser(ctx, "u1")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2 (only u1's)", len(list))
	}
	// Default first.
	if list[0].ID != a2.ID || !list[0].IsDefault {
		t.Errorf("default address should sort first, got %+v", list[0])
	}
	if list[1].ID != a1.ID {
		t.Errorf("second should be a1, got %s", list[1].ID)
	}
}

func TestMemoryAddresses_OwnershipScoped(t *testing.T) {
	ctx := context.Background()
	as := store.NewMemoryAddresses()
	a, _ := as.Create(ctx, sampleAddr("owner", false))

	// Another user can't see, edit, or delete it.
	if _, err := as.Get(ctx, "intruder", a.ID); !errors.Is(err, store.ErrAddressNotFound) {
		t.Errorf("Get(other user): want ErrAddressNotFound, got %v", err)
	}
	bad := a
	bad.UserID = "intruder"
	bad.City = "Lagos"
	if err := as.Update(ctx, bad); !errors.Is(err, store.ErrAddressNotFound) {
		t.Errorf("Update(other user): want ErrAddressNotFound, got %v", err)
	}
	if err := as.Delete(ctx, "intruder", a.ID); !errors.Is(err, store.ErrAddressNotFound) {
		t.Errorf("Delete(other user): want ErrAddressNotFound, got %v", err)
	}
	if err := as.SetDefault(ctx, "intruder", a.ID); !errors.Is(err, store.ErrAddressNotFound) {
		t.Errorf("SetDefault(other user): want ErrAddressNotFound, got %v", err)
	}
	// Owner is unaffected.
	if got, err := as.Get(ctx, "owner", a.ID); err != nil || got.City != "Jos" {
		t.Errorf("owner Get: got %+v err %v", got, err)
	}
}

func TestMemoryAddresses_SetDefaultIsExclusive(t *testing.T) {
	ctx := context.Background()
	as := store.NewMemoryAddresses()
	a1, _ := as.Create(ctx, sampleAddr("u1", true))
	a2, _ := as.Create(ctx, sampleAddr("u1", false))

	if err := as.SetDefault(ctx, "u1", a2.ID); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	g1, _ := as.Get(ctx, "u1", a1.ID)
	g2, _ := as.Get(ctx, "u1", a2.ID)
	if g1.IsDefault {
		t.Error("a1 should no longer be default")
	}
	if !g2.IsDefault {
		t.Error("a2 should be the default")
	}
	// Exactly one default in the list.
	list, _ := as.ListByUser(ctx, "u1")
	defaults := 0
	for _, a := range list {
		if a.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Errorf("want exactly 1 default, got %d", defaults)
	}
}

func TestMemoryAddresses_UpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	as := store.NewMemoryAddresses()
	a, _ := as.Create(ctx, sampleAddr("u1", false))

	upd := a
	upd.City = "Bukuru"
	if err := as.Update(ctx, upd); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got, _ := as.Get(ctx, "u1", a.ID); got.City != "Bukuru" {
		t.Errorf("city = %q, want Bukuru", got.City)
	}

	if err := as.Delete(ctx, "u1", a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := as.Get(ctx, "u1", a.ID); !errors.Is(err, store.ErrAddressNotFound) {
		t.Errorf("after delete: want ErrAddressNotFound, got %v", err)
	}
}
