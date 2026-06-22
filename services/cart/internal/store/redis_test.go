package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/menawar/ecommerce-platform/services/cart/internal/store"
)

// newStore spins up an IN-MEMORY Redis (miniredis) and points a real go-redis
// client at it. miniredis speaks the actual Redis protocol in-process, so these
// tests exercise our real HINCRBY/HSET/HDEL/EXPIRE calls with no Docker, no infra,
// and millisecond runtimes. RunT auto-closes it at test end.
func newStore(t *testing.T) (*store.Redis, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return store.NewRedis(client), mr
}

const user = "user-1"

func TestAddItem_AccumulatesAndGets(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)

	if _, err := s.AddItem(ctx, user, "p1", 2); err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	// Adding the same product again accumulates (HINCRBY).
	items, err := s.AddItem(ctx, user, "p1", 3)
	if err != nil {
		t.Fatalf("AddItem: %v", err)
	}
	if len(items) != 1 || items[0].ProductID != "p1" || items[0].Quantity != 5 {
		t.Fatalf("items = %+v, want p1 x5", items)
	}
}

func TestSetItem_OverwritesAndZeroRemoves(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	_, _ = s.AddItem(ctx, user, "p1", 5)

	items, _ := s.SetItem(ctx, user, "p1", 2) // absolute set
	if items[0].Quantity != 2 {
		t.Errorf("after SetItem(2) qty = %d, want 2", items[0].Quantity)
	}

	items, _ = s.SetItem(ctx, user, "p1", 0) // 0 removes the line
	if len(items) != 0 {
		t.Errorf("after SetItem(0) items = %+v, want empty", items)
	}
}

func TestAddItem_NegativeNetRemoves(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	_, _ = s.AddItem(ctx, user, "p1", 2)

	items, _ := s.AddItem(ctx, user, "p1", -5) // net -3 -> remove
	if len(items) != 0 {
		t.Errorf("items = %+v, want empty after net-negative", items)
	}
}

func TestRemoveAndClear(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	_, _ = s.AddItem(ctx, user, "p1", 1)
	_, _ = s.AddItem(ctx, user, "p2", 1)

	items, _ := s.RemoveItem(ctx, user, "p1")
	if len(items) != 1 || items[0].ProductID != "p2" {
		t.Errorf("after remove p1: %+v, want only p2", items)
	}

	if err := s.Clear(ctx, user); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	items, _ = s.Get(ctx, user)
	if len(items) != 0 {
		t.Errorf("after Clear: %+v, want empty", items)
	}
}

// TestItemsSortedDeterministically guards the stable ordering our API/tests rely on.
func TestItemsSorted(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	_, _ = s.AddItem(ctx, user, "p3", 1)
	_, _ = s.AddItem(ctx, user, "p1", 1)
	_, _ = s.AddItem(ctx, user, "p2", 1)

	items, _ := s.Get(ctx, user)
	got := []string{items[0].ProductID, items[1].ProductID, items[2].ProductID}
	if got[0] != "p1" || got[1] != "p2" || got[2] != "p3" {
		t.Errorf("order = %v, want [p1 p2 p3]", got)
	}
}

// TestTTLSetOnWrite proves the sliding expiry is applied — abandoned carts expire.
func TestTTLSetOnWrite(t *testing.T) {
	ctx := context.Background()
	s, mr := newStore(t)
	_, _ = s.AddItem(ctx, user, "p1", 1)

	ttl := mr.TTL("cart:" + user)
	if ttl <= 0 || ttl > 31*24*time.Hour {
		t.Errorf("ttl = %v, want a positive ~30d expiry", ttl)
	}
}
