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

// Token type constants carried in the "typ" claim, so an access token can't be
// replayed where a refresh token is required (and vice versa).
const (
	TypeAccess  = "access"
	TypeRefresh = "refresh"
)

// Claims is the trusted identity extracted from a validated token. It exposes
// only what callers need (who, role, the token's jti, and its type) — not the
// raw JWT internals.
type Claims struct {
	UserID  string // the "sub" claim
	Role    string // "customer" | "admin"
	TokenID string // the "jti" claim — the handle for server-side revocation
	Type    string // "access" | "refresh"
}

// TokenIssuer mints signed tokens. It returns the jti so the caller can persist
// it (refresh tokens are tracked server-side for revocation).
type TokenIssuer interface {
	Issue(userID, role string) (token, jti string, expiresAt time.Time, err error)
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
	Typ  string `json:"typ"` // "access" | "refresh"
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
	typ    string // the token type this manager issues + accepts (access | refresh)
}

// NewJWTManager builds a manager. secret comes from the JWT_SECRET env var at
// the edge of the program — this package never reads the environment itself
// (pkg/ stays free of process concerns).
func NewJWTManager(secret string, ttl time.Duration, typ string) *JWTManager {
	return &JWTManager{secret: []byte(secret), ttl: ttl, typ: typ}
}

// Issue creates a signed token valid for the manager's TTL, tagged with its type.
// It returns the token, its jti (so refresh tokens can be tracked), and expiry.
func (m *JWTManager) Issue(userID, role string) (string, string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.ttl)
	jti := uuid.NewString()

	claims := jwtClaims{
		Role: role,
		Typ:  m.typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        jti, // jti: unique id, the handle for server-side revocation
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, jti, expiresAt, nil
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

	// Reject a token of the wrong type — the two managers share the secret, so the
	// signature alone can't distinguish an access token from a refresh token; the
	// "typ" claim is what stops one being used where the other is expected.
	if claims.Typ != m.typ {
		return Claims{}, fmt.Errorf("%w: wrong token type %q", ErrInvalidToken, claims.Typ)
	}

	return Claims{
		UserID:  claims.Subject,
		Role:    claims.Role,
		TokenID: claims.ID,
		Type:    claims.Typ,
	}, nil
}
