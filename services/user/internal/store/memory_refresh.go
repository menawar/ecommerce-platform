package store

import (
	"context"
	"sync"
	"time"
)

// MemoryRefreshTokens is a concurrency-safe in-memory RefreshTokenStore for tests
// and local runs — the same role Memory plays for users.
type MemoryRefreshTokens struct {
	mu    sync.RWMutex
	byJTI map[string]RefreshToken
}

func NewMemoryRefreshTokens() *MemoryRefreshTokens {
	return &MemoryRefreshTokens{byJTI: make(map[string]RefreshToken)}
}

var _ RefreshTokenStore = (*MemoryRefreshTokens)(nil)

func (m *MemoryRefreshTokens) Save(_ context.Context, t RefreshToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byJTI[t.JTI] = t
	return nil
}

func (m *MemoryRefreshTokens) Get(_ context.Context, jti string) (RefreshToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.byJTI[jti]
	if !ok {
		return RefreshToken{}, ErrRefreshNotFound
	}
	return t, nil
}

func (m *MemoryRefreshTokens) Revoke(_ context.Context, jti string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byJTI[jti]
	if !ok {
		return ErrRefreshNotFound
	}
	if t.RevokedAt == nil {
		now := time.Now()
		t.RevokedAt = &now
		m.byJTI[jti] = t
	}
	return nil
}

func (m *MemoryRefreshTokens) RevokeAllForUser(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for jti, t := range m.byJTI {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &now
			m.byJTI[jti] = t
		}
	}
	return nil
}
