package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

// Integration tests for the Postgres verification-token store and the user
// SetEmailVerified path. Need a migrated userdb (migration 000003); SKIP otherwise.
func TestPostgres_VerificationTokenRoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := userPool(t)
	if _, err := pool.Exec(ctx, "TRUNCATE verification_tokens"); err != nil {
		t.Fatalf("truncate verification_tokens: %v", err)
	}
	vts := store.NewPostgresVerificationTokens(pool)

	tok := store.VerificationToken{
		Token: uuid.NewString(), UserID: uuid.NewString(),
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}
	if err := vts.Save(ctx, tok); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := vts.Get(ctx, tok.Token)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UserID != tok.UserID || got.UsedAt != nil || !got.Usable(time.Now()) {
		t.Errorf("got %+v, want usable token for %s", got, tok.UserID)
	}

	if err := vts.Use(ctx, tok.Token); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if got, _ = vts.Get(ctx, tok.Token); got.UsedAt == nil {
		t.Error("UsedAt should be set after Use")
	}

	t.Run("unknown token is ErrVerificationNotFound", func(t *testing.T) {
		if _, err := vts.Get(ctx, uuid.NewString()); !errors.Is(err, store.ErrVerificationNotFound) {
			t.Errorf("Get(random): want ErrVerificationNotFound, got %v", err)
		}
		if _, err := vts.Get(ctx, "not-a-uuid"); !errors.Is(err, store.ErrVerificationNotFound) {
			t.Errorf("Get(malformed): want ErrVerificationNotFound, got %v", err)
		}
		// Use is best-effort: an unknown token is a silent no-op, not an error.
		if err := vts.Use(ctx, uuid.NewString()); err != nil {
			t.Errorf("Use(unknown): want nil, got %v", err)
		}
	})
}

func TestPostgres_SetEmailVerified(t *testing.T) {
	ctx := context.Background()
	repo := store.NewPostgres(userPool(t))
	u := dbUser("verify@example.com")
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if got, _ := repo.GetByID(ctx, u.ID); got.EmailVerified {
		t.Fatal("new account should start unverified")
	}
	if err := repo.SetEmailVerified(ctx, u.ID); err != nil {
		t.Fatalf("SetEmailVerified: %v", err)
	}
	if got, _ := repo.GetByID(ctx, u.ID); !got.EmailVerified {
		t.Error("account should be verified after SetEmailVerified")
	}
}
