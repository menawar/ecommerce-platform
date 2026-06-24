package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ErrInvalidToken is returned for ANY token that fails validation — bad
// signature, expired, malformed, wrong algorithm. We deliberately collapse the
// reasons into one sentinel so the API never tells an attacker *why* their token
// was rejected, while callers can still branch with errors.Is to return 401.
var ErrInvalidToken = errors.New("auth: invalid token")

// Claims is the trusted identity extracted from a validated token. It exposes
// only what callers need (who, and what role) — not the raw JWT internals.
type Claims struct {
	UserID string // the "sub" claim
	Role   string // "customer" | "admin"
}

// TokenIssuer mints signed access tokens.
type TokenIssuer interface {
	Issue(userID, role string) (token string, expiresAt time.Time, err error)
}

// TokenValidator validates a token and returns its claims. The Gateway depends
// on THIS interface, not on *JWTManager — so it can validate locally (this type)
// or remotely (a client calling the User service's ValidateToken RPC) with no
// code change. Interfaces for the validator are the seam that keeps the Gateway
// decoupled from how identity is checked.
type TokenValidator interface {
	Validate(token string) (Claims, error)
}

// Compile-time proof JWTManager satisfies both roles.
var (
	_ TokenIssuer    = (*JWTManager)(nil)
	_ TokenValidator = (*JWTManager)(nil)
)

// jwtClaims is the on-the-wire claim set: our custom "role" plus the JWT
// registered claims (sub, exp, iat, jti). Embedding RegisteredClaims gives us
// the standard fields AND the library's built-in exp/iat validation for free.
type jwtClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// JWTManager issues and validates HS256 tokens for a single token kind (one TTL).
// The User service constructs two — one for access (15m), one for refresh (7d) —
// sharing the secret. Symmetric HS256 means the same secret signs and verifies,
// which is why ONLY the User service should hold it (the Gateway asks the User
// service to validate rather than holding the secret itself).
type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

// NewJWTManager builds a manager. secret comes from the JWT_SECRET env var at
// the edge of the program — this package never reads the environment itself
// (pkg/ stays free of process concerns).
func NewJWTManager(secret string, ttl time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), ttl: ttl}
}

// Issue creates a signed token valid for the manager's TTL.
func (m *JWTManager) Issue(userID, role string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.ttl)

	claims := jwtClaims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(), // jti: unique id, enables future revocation
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, expiresAt, nil
}

// Validate parses and verifies a token, returning its trusted claims.
func (m *JWTManager) Validate(tokenString string) (Claims, error) {
	var claims jwtClaims

	_, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		// THE critical check: confirm the token's algorithm is the HMAC family we
		// expect. Without it, an attacker can swap alg to "none" (no signature) or
		// trick an RS256 verifier into treating the public key as an HMAC secret —
		// the classic algorithm-confusion attack. Never trust t.Header["alg"].
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"})) // defense-in-depth: reject other algs before keyfunc
	if err != nil {
		// Collapse every failure reason into the sentinel; keep the detail wrapped
		// for our own logs, not for the caller's response.
		return Claims{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	return Claims{UserID: claims.Subject, Role: claims.Role}, nil
}
