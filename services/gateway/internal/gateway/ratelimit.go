package gateway

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

// rateLimit is middleware that spends one token per request from a Redis token
// bucket. It keys on the authenticated user when available (so one account can't
// hammer the API) and otherwise on the client IP (so unauthenticated abuse — login
// brute force, scraping — is bounded). Because identity is only in the context
// AFTER requireAuth, this middleware is mounted inside the protected groups for
// per-user keying and on the public routes for per-IP keying.
//
// It is a no-op when no limiter is configured (unit tests), and FAILS OPEN if Redis
// errors: a cache blip should not take the whole edge down. A breach is logged.
func (h *Handler) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.limiter == nil {
			next.ServeHTTP(w, r)
			return
		}

		key := rateLimitKey(r)
		res, err := h.limiter.Allow(r.Context(), key)
		if err != nil {
			// Fail open — but make it visible so a persistent Redis outage is caught.
			h.log.ErrorContext(r.Context(), "rate limiter unavailable, allowing request", "err", err, "key", key)
			next.ServeHTTP(w, r)
			return
		}
		if !res.Allowed {
			retry := max(int(res.RetryAfter.Seconds()), 1)
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded; slow down")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimitKey is the bucket key for this request: the user id if authenticated,
// else the client IP. The prefixes keep the two namespaces from colliding.
func rateLimitKey(r *http.Request) string {
	if id, ok := IdentityFrom(r.Context()); ok && id.UserID != "" {
		return "rl:user:" + id.UserID
	}
	return "rl:ip:" + clientIP(r)
}

// clientIP resolves the caller's address. Behind the trusted BFF/proxy the real
// client is in X-Forwarded-For (first hop); otherwise fall back to the socket peer.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Left-most entry is the original client; trim any subsequent proxy hops.
		first, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
