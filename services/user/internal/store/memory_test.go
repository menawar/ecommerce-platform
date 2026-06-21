package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

// Compile-time proof that *Memory satisfies the Repository interface. If a
// method signature ever drifts, the build breaks here instead of at a call site.
var _ store.Repository = (*store.Memory)(nil)

func sampleUser(id, email string) store.User {
	return store.User{ID: id, Email: email, PasswordHash: "hash", FullName: "Test", Role: "customer"}
}

func TestMemory_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()

	if err := repo.Create(ctx, sampleUser("id-1", "ada@example.com")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	t.Run("GetByEmail is case-insensitive (emulates CITEXT)", func(t *testing.T) {
		got, err := repo.GetByEmail(ctx, "ADA@example.com")
		if err != nil {
			t.Fatalf("GetByEmail: %v", err)
		}
		if got.ID != "id-1" {
			t.Errorf("got id %q, want id-1", got.ID)
		}
	})

	t.Run("GetByID returns the account", func(t *testing.T) {
		got, err := repo.GetByID(ctx, "id-1")
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.Email != "ada@example.com" {
			t.Errorf("got email %q", got.Email)
		}
	})
}

func TestMemory_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()
	_ = repo.Create(ctx, sampleUser("id-1", "dup@example.com"))

	// Same email, different case, different id -> still a duplicate.
	err := repo.Create(ctx, sampleUser("id-2", "DUP@example.com"))
	if !errors.Is(err, store.ErrEmailTaken) {
		t.Errorf("want ErrEmailTaken, got %v", err)
	}
}

func TestMemory_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()

	if _, err := repo.GetByEmail(ctx, "nobody@example.com"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetByEmail: want ErrNotFound, got %v", err)
	}
	if _, err := repo.GetByID(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetByID: want ErrNotFound, got %v", err)
	}
}

// TestMemory_ConcurrentDistinct fires many goroutines creating DIFFERENT users.
// Run with -race, it proves the RWMutex actually protects the maps: without the
// lock the race detector flags concurrent map writes and the test fails.
func TestMemory_ConcurrentDistinct(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("id-%d", i)
			email := fmt.Sprintf("user%d@example.com", i)
			if err := repo.Create(ctx, sampleUser(id, email)); err != nil {
				t.Errorf("Create(%s): %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if _, err := repo.GetByID(ctx, fmt.Sprintf("id-%d", i)); err != nil {
			t.Errorf("missing id-%d after concurrent insert: %v", i, err)
		}
	}
}

// TestMemory_ConcurrentSameEmail fires many goroutines at the SAME email with
// distinct ids. Exactly one must win; the rest get ErrEmailTaken. This proves
// the check-then-insert is atomic (no double-registration under contention) — a
// preview of the Phase 2 concurrent-stock-reservation test.
func TestMemory_ConcurrentSameEmail(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()

	const n = 50
	var success int64
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			err := repo.Create(ctx, sampleUser(fmt.Sprintf("id-%d", i), "race@example.com"))
			switch {
			case err == nil:
				atomic.AddInt64(&success, 1)
			case errors.Is(err, store.ErrEmailTaken):
				// expected for the losers
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if success != 1 {
		t.Errorf("want exactly 1 successful registration, got %d", success)
	}
}
