package store

import (
	"context"
	"sync"
	"time"
)

// MemoryPasswordResetTokens is a concurrency-safe in-memory PasswordResetTokenStore
// for tests and local runs — the same role MemoryVerificationTokens plays.
type MemoryPasswordResetTokens struct {
	mu      sync.RWMutex
	byToken map[string]PasswordResetToken
}

func NewMemoryPasswordResetTokens() *MemoryPasswordResetTokens {
	return &MemoryPasswordResetTokens{byToken: make(map[string]PasswordResetToken)}
}

var _ PasswordResetTokenStore = (*MemoryPasswordResetTokens)(nil)

func (m *MemoryPasswordResetTokens) Save(_ context.Context, t PasswordResetToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byToken[t.Token] = t
	return nil
}

func (m *MemoryPasswordResetTokens) Get(_ context.Context, token string) (PasswordResetToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.byToken[token]
	if !ok {
		return PasswordResetToken{}, ErrPasswordResetNotFound
	}
	return t, nil
}

// Consume flips an unused token to used under the lock and reports whether THIS
// call performed the flip — so two concurrent callers can't both win (matches the
// Postgres conditional UPDATE's rows-affected semantics).
func (m *MemoryPasswordResetTokens) Consume(_ context.Context, token string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byToken[token]
	if !ok || t.UsedAt != nil {
		return false, nil
	}
	now := time.Now()
	t.UsedAt = &now
	m.byToken[token] = t
	return true, nil
}

func (m *MemoryPasswordResetTokens) InvalidateForUser(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for tok, t := range m.byToken {
		if t.UserID == userID && t.UsedAt == nil {
			t.UsedAt = &now
			m.byToken[tok] = t
		}
	}
	return nil
}
