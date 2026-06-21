package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/menawar/ecommerce-platform/pkg/auth"
)

const testSecret = "test-secret-do-not-use-in-prod"

// TestJWT_RoundTrip proves an issued token validates back to the same identity.
func TestJWT_RoundTrip(t *testing.T) {
	m := auth.NewJWTManager(testSecret, 15*time.Minute)

	token, expiresAt, err := m.Issue("user-123", "admin")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if !expiresAt.After(time.Now()) {
		t.Errorf("expiresAt %v is not in the future", expiresAt)
	}

	claims, err := m.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.UserID != "user-123" || claims.Role != "admin" {
		t.Errorf("claims = %+v, want {user-123 admin}", claims)
	}
}

// TestJWT_Tampered proves a flipped byte breaks signature verification.
func TestJWT_Tampered(t *testing.T) {
	m := auth.NewJWTManager(testSecret, 15*time.Minute)
	token, _, _ := m.Issue("user-123", "customer")

	// Mutate the last char of the signature segment.
	tampered := token[:len(token)-1] + flip(token[len(token)-1])

	if _, err := m.Validate(tampered); !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("want ErrInvalidToken for tampered token, got %v", err)
	}
}

// TestJWT_Expired proves expiry is enforced. We issue with a negative TTL so the
// token is already past its exp.
func TestJWT_Expired(t *testing.T) {
	m := auth.NewJWTManager(testSecret, -time.Minute)
	token, _, _ := m.Issue("user-123", "customer")

	if _, err := m.Validate(token); !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("want ErrInvalidToken for expired token, got %v", err)
	}
}

// TestJWT_WrongSecret proves a token signed by a different secret is rejected —
// the validator trusts only tokens it could have signed.
func TestJWT_WrongSecret(t *testing.T) {
	issuer := auth.NewJWTManager("secret-A", 15*time.Minute)
	verifier := auth.NewJWTManager("secret-B", 15*time.Minute)

	token, _, _ := issuer.Issue("user-123", "customer")
	if _, err := verifier.Validate(token); !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("want ErrInvalidToken for foreign secret, got %v", err)
	}
}

// TestJWT_AlgNone proves the algorithm-confusion defense: a token with alg=none
// (no signature at all) must be rejected even though it's structurally valid.
func TestJWT_AlgNone(t *testing.T) {
	m := auth.NewJWTManager(testSecret, 15*time.Minute)

	// Craft an unsigned token. The library requires an explicit unsafe sentinel
	// to even produce one — which is exactly the attack we must reject on verify.
	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Subject:   "attacker",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	tokenString, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("crafting none-token: %v", err)
	}

	if _, err := m.Validate(tokenString); !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("want ErrInvalidToken for alg=none token, got %v", err)
	}
}

// flip returns a different ASCII char so the mutated token is guaranteed changed.
func flip(b byte) string {
	if b == 'A' {
		return "B"
	}
	return "A"
}
