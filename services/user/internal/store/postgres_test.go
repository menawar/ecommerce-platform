package store_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	pgpool "github.com/menawar/ecommerce-platform/pkg/postgres"
	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

// Integration tests for the Postgres store. Need a migrated userdb; SKIP otherwise.
func userPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("USER_DB_URL")
	if url == "" {
		url = "postgres://ecommerce:ecommerce@localhost:5433/userdb?sslmode=disable"
	}
	pool, err := pgpool.NewPool(context.Background(), url)
	if err != nil {
		t.Skipf("skipping integration test (userdb unavailable; run `make infra-up && make user-migrate-up`): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "TRUNCATE users"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

func dbUser(email string) store.User {
	now := time.Now().UTC()
	return store.User{
		ID: uuid.NewString(), Email: email, PasswordHash: "hash",
		FullName: "Test User", Role: "customer", CreatedAt: now, UpdatedAt: now,
	}
}

func TestPostgres_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	repo := store.NewPostgres(userPool(t))
	u := dbUser("ada@example.com")

	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	t.Run("GetByEmail is case-insensitive (CITEXT)", func(t *testing.T) {
		got, err := repo.GetByEmail(ctx, "ADA@example.com")
		if err != nil {
			t.Fatalf("GetByEmail: %v", err)
		}
		if got.ID != u.ID || got.Email != "ada@example.com" || got.Role != "customer" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("GetByID", func(t *testing.T) {
		got, err := repo.GetByID(ctx, u.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.Email != "ada@example.com" || got.PasswordHash != "hash" {
			t.Errorf("got %+v", got)
		}
	})
}

func TestPostgres_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	repo := store.NewPostgres(userPool(t))

	if err := repo.Create(ctx, dbUser("dup@example.com")); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	// Different case, different id -> still a duplicate via CITEXT UNIQUE.
	if err := repo.Create(ctx, dbUser("DUP@example.com")); !errors.Is(err, store.ErrEmailTaken) {
		t.Errorf("want ErrEmailTaken, got %v", err)
	}
}

func TestPostgres_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := store.NewPostgres(userPool(t))

	if _, err := repo.GetByEmail(ctx, "nobody@example.com"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetByEmail: want ErrNotFound, got %v", err)
	}
	if _, err := repo.GetByID(ctx, uuid.NewString()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetByID(random): want ErrNotFound, got %v", err)
	}
	if _, err := repo.GetByID(ctx, "not-a-uuid"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetByID(malformed): want ErrNotFound, got %v", err)
	}
}
