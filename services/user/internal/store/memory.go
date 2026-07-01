package store

import (
	"context"
	"strings"
	"sync"
)

// Memory is a concurrency-safe, in-memory Repository. It lets Phase 1 ship
// without a database; Phase 2 replaces it with a Postgres implementation behind
// the same Repository interface.
//
// Two maps give O(1) lookup by both id and email. byEmail keys are LOWERCASED,
// which emulates the spec's `email CITEXT UNIQUE` column: "A@x.com" and
// "a@x.com" are the same account.
type Memory struct {
	// A plain map is NOT safe for concurrent use — concurrent writes (or a write
	// racing a read) can corrupt it or panic. The RWMutex guards every access:
	// many concurrent readers OR one writer, never both. We need this because a
	// gRPC server handles requests on many goroutines at once.
	mu      sync.RWMutex
	byID    map[string]User
	byEmail map[string]string // lowercased email -> id
	deleted map[string]bool   // ids that have been erased (idempotency marker)
}

// NewMemory returns an initialized, empty in-memory repository. Returning the
// concrete *Memory (not the interface) is idiomatic: "accept interfaces, return
// structs" — the caller decides to hold it as a store.Repository.
func NewMemory() *Memory {
	return &Memory{
		byID:    make(map[string]User),
		byEmail: make(map[string]string),
		deleted: make(map[string]bool),
	}
}

func normalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}

// Create inserts u, rejecting a duplicate email. The check-then-insert happens
// under a single write lock so two goroutines registering the same email can't
// both pass the existence check — exactly one wins, the other gets ErrEmailTaken.
func (m *Memory) Create(_ context.Context, u User) error {
	key := normalizeEmail(u.Email)

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.byEmail[key]; exists {
		return ErrEmailTaken
	}
	m.byID[u.ID] = u
	m.byEmail[key] = u.ID
	return nil
}

// DeleteAccount anonymises the user and frees their email (so login by it fails
// and the address can be re-registered). Idempotent. The in-memory double only
// tombstones the user record; the Postgres store additionally purges addresses and
// tokens in one transaction.
func (m *Memory) DeleteAccount(_ context.Context, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	u, ok := m.byID[id]
	if !ok || m.deleted[id] {
		return false, nil // unknown or already deleted
	}
	delete(m.byEmail, normalizeEmail(u.Email))
	u.Email = "deleted+" + id + "@deleted.invalid"
	u.PasswordHash = ""
	u.FullName = ""
	u.EmailVerified = false
	m.byID[id] = u
	m.deleted[id] = true
	return true, nil
}

func (m *Memory) GetByEmail(_ context.Context, email string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.byEmail[normalizeEmail(email)]
	if !ok {
		return User{}, ErrNotFound
	}
	return m.byID[id], nil
}

func (m *Memory) GetByID(_ context.Context, id string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.byID[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

// SetEmailVerified flips the flag for an existing account. Verifying an unknown
// id is a no-op (mirrors the Postgres UPDATE affecting zero rows), and verifying
// an already-verified account is idempotent.
func (m *Memory) SetEmailVerified(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if u, ok := m.byID[userID]; ok {
		u.EmailVerified = true
		m.byID[userID] = u
	}
	return nil
}

// UpdatePassword replaces the account's password hash. An unknown id is a no-op
// (mirrors the Postgres UPDATE affecting zero rows).
func (m *Memory) UpdatePassword(_ context.Context, userID, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if u, ok := m.byID[userID]; ok {
		u.PasswordHash = passwordHash
		m.byID[userID] = u
	}
	return nil
}
