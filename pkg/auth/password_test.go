package auth_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/menawar/ecommerce-platform/pkg/auth"
)

// TestHashPassword_NotPlaintext proves we never store the raw password: the hash
// must differ from the input and look like a bcrypt string ($2a$...).
func TestHashPassword_NotPlaintext(t *testing.T) {
	const pw = "correct horse battery staple"

	hash, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == pw {
		t.Fatal("hash equals plaintext — password stored in the clear")
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Errorf("hash %q is not a bcrypt hash", hash)
	}
}

// TestHashPassword_Salted proves the salt is random: hashing the same password
// twice yields different hashes, so identical passwords aren't detectable in the
// database (defeats rainbow tables).
func TestHashPassword_Salted(t *testing.T) {
	h1, _ := auth.HashPassword("same-password")
	h2, _ := auth.HashPassword("same-password")
	if h1 == h2 {
		t.Error("two hashes of the same password are identical — salt is not random")
	}
}

// TestVerifyPassword covers the three outcomes a caller must distinguish:
// match (nil), wrong password (ErrPasswordMismatch sentinel), malformed hash
// (some other error). The wrong-password case MUST be the sentinel so the
// service can map it to 401, not 500.
func TestVerifyPassword(t *testing.T) {
	hash, err := auth.HashPassword("s3cret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	t.Run("correct password returns nil", func(t *testing.T) {
		if err := auth.VerifyPassword(hash, "s3cret"); err != nil {
			t.Errorf("want nil, got %v", err)
		}
	})

	t.Run("wrong password returns ErrPasswordMismatch", func(t *testing.T) {
		err := auth.VerifyPassword(hash, "guess")
		if !errors.Is(err, auth.ErrPasswordMismatch) {
			t.Errorf("want ErrPasswordMismatch, got %v", err)
		}
	})

	t.Run("malformed hash returns a non-sentinel error", func(t *testing.T) {
		err := auth.VerifyPassword("not-a-bcrypt-hash", "s3cret")
		if err == nil || errors.Is(err, auth.ErrPasswordMismatch) {
			t.Errorf("want a real error (not mismatch sentinel), got %v", err)
		}
	})
}
