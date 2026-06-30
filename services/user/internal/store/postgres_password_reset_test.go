package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

// Integration tests for the Postgres password-reset store and UpdatePassword.
// Need a migrated userdb (migration 000004); SKIP otherwise.
func TestPostgres_PasswordResetTokenRoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := userPool(t)
	if _, err := pool.Exec(ctx, "TRUNCATE password_reset_tokens"); err != nil {
		t.Fatalf("truncate password_reset_tokens: %v", err)
	}
	prs := store.NewPostgresPasswordResetTokens(pool)

	tok := store.PasswordResetToken{
		Token: uuid.NewString(), UserID: uuid.NewString(),
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}
	if err := prs.Save(ctx, tok); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := prs.Get(ctx, tok.Token)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UserID != tok.UserID || !got.Usable(time.Now()) {
		t.Errorf("got %+v, want usable token for %s", got, tok.UserID)
	}

	won, err := prs.Consume(ctx, tok.Token)
	if err != nil || !won {
		t.Fatalf("Consume: won=%v err=%v, want true/nil", won, err)
	}
	if got, _ = prs.Get(ctx, tok.Token); got.UsedAt == nil {
		t.Error("UsedAt should be set after Consume")
	}
	// Second consume loses the single-use race.
	if won, err := prs.Consume(ctx, tok.Token); won || err != nil {
		t.Errorf("second Consume: won=%v err=%v, want false/nil", won, err)
	}

	t.Run("unknown token", func(t *testing.T) {
		if _, err := prs.Get(ctx, uuid.NewString()); !errors.Is(err, store.ErrPasswordResetNotFound) {
			t.Errorf("Get(random): want ErrPasswordResetNotFound, got %v", err)
		}
		if won, err := prs.Consume(ctx, uuid.NewString()); won || err != nil {
			t.Errorf("Consume(unknown): won=%v err=%v, want false/nil", won, err)
		}
	})
}

func TestPostgres_UpdatePassword(t *testing.T) {
	ctx := context.Background()
	repo := store.NewPostgres(userPool(t))
	u := dbUser("reset@example.com")
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdatePassword(ctx, u.ID, "new-hash"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}
	got, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.PasswordHash != "new-hash" {
		t.Errorf("password hash = %q, want new-hash", got.PasswordHash)
	}
}
