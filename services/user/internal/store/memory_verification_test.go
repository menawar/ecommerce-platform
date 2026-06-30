package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

// Compile-time proof that the in-memory type satisfies the port.
var _ store.VerificationTokenStore = (*store.MemoryVerificationTokens)(nil)

func TestVerificationToken_Usable(t *testing.T) {
	now := time.Now()
	used := now.Add(-time.Minute)
	cases := []struct {
		name string
		tok  store.VerificationToken
		want bool
	}{
		{"fresh and unused", store.VerificationToken{ExpiresAt: now.Add(time.Hour)}, true},
		{"expired", store.VerificationToken{ExpiresAt: now.Add(-time.Hour)}, false},
		{"already used", store.VerificationToken{ExpiresAt: now.Add(time.Hour), UsedAt: &used}, false},
		{"used and expired", store.VerificationToken{ExpiresAt: now.Add(-time.Hour), UsedAt: &used}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.tok.Usable(now); got != c.want {
				t.Errorf("Usable() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestMemoryVerificationTokens_RoundTrip(t *testing.T) {
	ctx := context.Background()
	vts := store.NewMemoryVerificationTokens()

	tok := store.VerificationToken{Token: "tok-1", UserID: "user-1", ExpiresAt: time.Now().Add(time.Hour)}
	if err := vts.Save(ctx, tok); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := vts.Get(ctx, "tok-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UserID != "user-1" || got.UsedAt != nil {
		t.Errorf("got %+v, want unused token for user-1", got)
	}
	if !got.Usable(time.Now()) {
		t.Error("freshly saved token should be Usable")
	}

	if err := vts.Use(ctx, "tok-1"); err != nil {
		t.Fatalf("Use: %v", err)
	}
	got, _ = vts.Get(ctx, "tok-1")
	if got.UsedAt == nil {
		t.Fatal("UsedAt should be set after Use")
	}
	firstUsedAt := *got.UsedAt

	// A second Use is a no-op: used_at must not move (mirrors the Postgres guard).
	if err := vts.Use(ctx, "tok-1"); err != nil {
		t.Fatalf("second Use: %v", err)
	}
	got, _ = vts.Get(ctx, "tok-1")
	if !got.UsedAt.Equal(firstUsedAt) {
		t.Errorf("second Use moved used_at: %v -> %v", firstUsedAt, *got.UsedAt)
	}
}

func TestMemoryVerificationTokens_NotFound(t *testing.T) {
	ctx := context.Background()
	vts := store.NewMemoryVerificationTokens()

	if _, err := vts.Get(ctx, "missing"); !errors.Is(err, store.ErrVerificationNotFound) {
		t.Errorf("Get: want ErrVerificationNotFound, got %v", err)
	}
	// Use is best-effort: a missing token is a silent no-op (matches Postgres).
	if err := vts.Use(ctx, "missing"); err != nil {
		t.Errorf("Use(missing): want nil (best-effort no-op), got %v", err)
	}
}

func TestMemory_SetEmailVerified(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()
	if err := repo.Create(ctx, sampleUser("id-1", "ada@example.com")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if u, _ := repo.GetByID(ctx, "id-1"); u.EmailVerified {
		t.Fatal("new account should start unverified")
	}

	if err := repo.SetEmailVerified(ctx, "id-1"); err != nil {
		t.Fatalf("SetEmailVerified: %v", err)
	}
	if u, _ := repo.GetByID(ctx, "id-1"); !u.EmailVerified {
		t.Error("account should be verified after SetEmailVerified")
	}

	// Idempotent, and a no-op (not an error) for an unknown id.
	if err := repo.SetEmailVerified(ctx, "id-1"); err != nil {
		t.Errorf("repeat SetEmailVerified: %v", err)
	}
	if err := repo.SetEmailVerified(ctx, "missing"); err != nil {
		t.Errorf("SetEmailVerified(unknown): want no-op, got %v", err)
	}
}
