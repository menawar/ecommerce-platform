// Package auth holds authentication primitives shared across services: password
// hashing now, JWT issuance/validation next. It carries no service-specific
// logic and depends only on golang.org/x/crypto — so any service can import it
// without dragging in another service's concerns.
package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// ErrPasswordMismatch is returned when a plaintext password does not match a
// stored hash. It is a NORMAL, expected outcome (a user typo'd their password),
// not a server fault — callers should map it to "unauthorized", never a 500.
// Exposing it as a sentinel lets callers branch with errors.Is.
var ErrPasswordMismatch = errors.New("auth: password does not match hash")

// DefaultCost is the bcrypt work factor. Each +1 roughly DOUBLES the time to
// hash (and to brute-force). 10 (~60ms) balances login latency vs. attacker
// cost; bump it as hardware gets faster.
const DefaultCost = bcrypt.DefaultCost

// HashPassword returns a bcrypt hash of the plaintext password. bcrypt generates
// a random salt internally and embeds it (plus the cost) INTO the returned
// string, so you store exactly this one value — there is no separate salt column
// to manage and no way to forget to salt.
func HashPassword(plaintext string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plaintext), DefaultCost)
	if err != nil {
		// Wrap with %w so callers can still inspect the underlying cause, while
		// adding context about where it happened.
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(b), nil
}

// VerifyPassword reports whether plaintext matches a previously stored hash.
// Returns nil on a match, ErrPasswordMismatch on a wrong password, or a wrapped
// error if the stored hash is malformed (a data/programmer error, not a user
// one). The comparison is constant-time inside bcrypt, so it leaks no timing
// signal about how many leading characters were correct.
func VerifyPassword(hash, plaintext string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
	switch {
	case err == nil:
		return nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return ErrPasswordMismatch
	default:
		return fmt.Errorf("auth: verify password: %w", err)
	}
}
