package auth_test

import (
	"errors"
	"strings"
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

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT segments, got %d", len(parts))
	}
	// Tamper the FIRST byte of the signature segment. We must NOT flip the last
	// base64url char: a 32-byte HMAC encodes to 43 base64url chars whose final
	// char has 2 unused low bits, so some flips decode to the SAME signature and
	// the token still validates (the flaky bug this test originally had). The
	// first char always carries meaningful bits, so changing it always changes
	// the decoded signature.
	sig := []byte(parts[2])
	sig[0] = flipB64(sig[0])
	parts[2] = string(sig)
	tampered := strings.Join(parts, ".")

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

// flipB64 returns a different base64url character, so the byte it encodes is
// guaranteed to change.
func flipB64(c byte) byte {
	if c == 'A' {
		return 'B'
	}
	return 'A'
}
