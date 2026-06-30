package store

import (
	"context"
	"sync"
	"time"
)

// MemoryVerificationTokens is a concurrency-safe in-memory VerificationTokenStore
// for tests and local runs — the same role MemoryRefreshTokens plays for refresh
// tokens.
type MemoryVerificationTokens struct {
	mu      sync.RWMutex
	byToken map[string]VerificationToken
}

func NewMemoryVerificationTokens() *MemoryVerificationTokens {
	return &MemoryVerificationTokens{byToken: make(map[string]VerificationToken)}
}

var _ VerificationTokenStore = (*MemoryVerificationTokens)(nil)

func (m *MemoryVerificationTokens) Save(_ context.Context, t VerificationToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byToken[t.Token] = t
	return nil
}

func (m *MemoryVerificationTokens) Get(_ context.Context, token string) (VerificationToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.byToken[token]
	if !ok {
		return VerificationToken{}, ErrVerificationNotFound
	}
	return t, nil
}

// Use marks the token consumed only if it was still unused, so a concurrent
// double-verify sets used_at at most once — matching the Postgres UPDATE guard.
// Best-effort per the interface: a missing or already-used token is a silent
// no-op (the Postgres :exec UPDATE likewise can't distinguish those), so the two
// implementations agree.
func (m *MemoryVerificationTokens) Use(_ context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byToken[token]
	if !ok {
		return nil
	}
	if t.UsedAt == nil {
		now := time.Now()
		t.UsedAt = &now
		m.byToken[token] = t
	}
	return nil
}
