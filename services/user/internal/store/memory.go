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
}

// NewMemory returns an initialized, empty in-memory repository. Returning the
// concrete *Memory (not the interface) is idiomatic: "accept interfaces, return
// structs" — the caller decides to hold it as a store.Repository.
func NewMemory() *Memory {
	return &Memory{
		byID:    make(map[string]User),
		byEmail: make(map[string]string),
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
