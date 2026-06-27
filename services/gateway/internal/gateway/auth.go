package gateway

import (
	"context"
	"net/http"
	"strings"

	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
)

// ctxKey is an UNEXPORTED type used as a context key. Using a private type (not a
// string) makes the key impossible to collide with or read from another package —
// only code in this package can set or get the value. This is the standard way
// to carry request-scoped values through context.
type ctxKey int

const identityKey ctxKey = iota

// Identity is the authenticated caller, derived from a validated token. Handlers
// downstream of requireAuth read it from the context instead of re-parsing the
// token.
type Identity struct {
	UserID string
	Role   string
}

// IdentityFrom returns the caller's identity if requireAuth ran on this request.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

// requireAuth is middleware that gates a route behind a valid access token. It
// pulls the Bearer token, asks the User service to validate it, and on success
// stores the Identity in the request context for downstream handlers. This is
// the same job a gRPC interceptor does on the server side — middleware is just
// the HTTP-edge equivalent.
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
			return
		}

		// Validation lives in the User service (it holds the signing secret), so
		// the gateway asks over gRPC rather than verifying locally.
		resp, err := h.users.ValidateToken(r.Context(), &userv1.ValidateTokenRequest{Token: token})
		if err != nil {
			// The validate CALL itself failed (e.g. user service down). That's an
			// upstream error, NOT "your token is bad" — map it as such (503/500).
			h.writeGRPCError(w, r, err)
			return
		}
		if !resp.GetValid() {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), identityKey, Identity{
			UserID: resp.GetUserId(),
			Role:   resp.GetRole(),
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireAdmin gates a route behind the admin role. It is meant to be chained
// AFTER requireAuth, which has already validated the token and stored the
// Identity — so this middleware only inspects the role. Splitting "are you
// authenticated" (requireAuth, 401) from "are you authorized" (requireAdmin, 403)
// keeps each check single-purpose and lets routes opt into either or both.
func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := IdentityFrom(r.Context())
		if !ok {
			// No Identity means requireAuth didn't run ahead of us — a routing
			// mistake. Fail closed (401) rather than silently allow the request.
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if id.Role != "admin" {
			writeError(w, http.StatusForbidden, "admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bearerToken extracts the token from an "Authorization: Bearer <token>" header.
// The scheme match is case-insensitive per RFC 7235; the token must be non-empty.
func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if len(header) < len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}
