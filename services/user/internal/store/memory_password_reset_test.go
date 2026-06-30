package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

var _ store.PasswordResetTokenStore = (*store.MemoryPasswordResetTokens)(nil)

func TestPasswordResetToken_Usable(t *testing.T) {
	now := time.Now()
	used := now.Add(-time.Minute)
	cases := []struct {
		name string
		tok  store.PasswordResetToken
		want bool
	}{
		{"fresh and unused", store.PasswordResetToken{ExpiresAt: now.Add(time.Hour)}, true},
		{"expired", store.PasswordResetToken{ExpiresAt: now.Add(-time.Hour)}, false},
		{"already used", store.PasswordResetToken{ExpiresAt: now.Add(time.Hour), UsedAt: &used}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.tok.Usable(now); got != c.want {
				t.Errorf("Usable() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestMemoryPasswordResetTokens_RoundTrip(t *testing.T) {
	ctx := context.Background()
	prs := store.NewMemoryPasswordResetTokens()

	tok := store.PasswordResetToken{Token: "tok-1", UserID: "user-1", ExpiresAt: time.Now().Add(time.Hour)}
	if err := prs.Save(ctx, tok); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := prs.Get(ctx, "tok-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UserID != "user-1" || !got.Usable(time.Now()) {
		t.Errorf("got %+v, want usable token for user-1", got)
	}

	won, err := prs.Consume(ctx, "tok-1")
	if err != nil || !won {
		t.Fatalf("Consume: won=%v err=%v, want true/nil", won, err)
	}
	got, _ = prs.Get(ctx, "tok-1")
	if got.UsedAt == nil {
		t.Fatal("UsedAt should be set after Consume")
	}

	// A second Consume loses the race: returns false (single-use enforced).
	won, err = prs.Consume(ctx, "tok-1")
	if err != nil || won {
		t.Errorf("second Consume: won=%v err=%v, want false/nil", won, err)
	}
}

func TestMemoryPasswordResetTokens_InvalidateForUser(t *testing.T) {
	ctx := context.Background()
	prs := store.NewMemoryPasswordResetTokens()
	exp := time.Now().Add(time.Hour)
	_ = prs.Save(ctx, store.PasswordResetToken{Token: "a", UserID: "u1", ExpiresAt: exp})
	_ = prs.Save(ctx, store.PasswordResetToken{Token: "b", UserID: "u1", ExpiresAt: exp})
	_ = prs.Save(ctx, store.PasswordResetToken{Token: "c", UserID: "u2", ExpiresAt: exp})

	if err := prs.InvalidateForUser(ctx, "u1"); err != nil {
		t.Fatalf("InvalidateForUser: %v", err)
	}
	// u1's tokens can no longer be consumed; u2's is untouched.
	for _, tok := range []string{"a", "b"} {
		if won, _ := prs.Consume(ctx, tok); won {
			t.Errorf("token %q should be invalidated, but Consume won", tok)
		}
	}
	if won, _ := prs.Consume(ctx, "c"); !won {
		t.Error("u2's token should still be consumable")
	}
}

func TestMemoryPasswordResetTokens_NotFound(t *testing.T) {
	ctx := context.Background()
	prs := store.NewMemoryPasswordResetTokens()

	if _, err := prs.Get(ctx, "missing"); !errors.Is(err, store.ErrPasswordResetNotFound) {
		t.Errorf("Get: want ErrPasswordResetNotFound, got %v", err)
	}
	// Consuming a missing token wins nothing (and is not an error).
	if won, err := prs.Consume(ctx, "missing"); won || err != nil {
		t.Errorf("Consume(missing): won=%v err=%v, want false/nil", won, err)
	}
}

func TestMemory_UpdatePassword(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory()
	if err := repo.Create(ctx, sampleUser("id-1", "ada@example.com")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdatePassword(ctx, "id-1", "new-hash"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	got, _ := repo.GetByID(ctx, "id-1")
	if got.PasswordHash != "new-hash" {
		t.Errorf("password hash = %q, want new-hash", got.PasswordHash)
	}

	// Unknown id is a no-op, not an error.
	if err := repo.UpdatePassword(ctx, "missing", "x"); err != nil {
		t.Errorf("UpdatePassword(unknown): want no-op, got %v", err)
	}
}
