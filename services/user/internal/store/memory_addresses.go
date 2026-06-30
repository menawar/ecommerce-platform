package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryAddresses is a concurrency-safe in-memory AddressStore for tests and local
// runs — the same role the other Memory* stores play.
type MemoryAddresses struct {
	mu    sync.RWMutex
	byID  map[string]Address
	order map[string]int64 // insertion sequence, for created_at-desc ordering
	seq   int64
}

func NewMemoryAddresses() *MemoryAddresses {
	return &MemoryAddresses{byID: make(map[string]Address), order: make(map[string]int64)}
}

var _ AddressStore = (*MemoryAddresses)(nil)

func (m *MemoryAddresses) Create(_ context.Context, a Address) (Address, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if a.IsDefault {
		m.clearDefault(a.UserID)
	}
	now := time.Now()
	a.ID = uuid.NewString()
	a.CreatedAt = now
	a.UpdatedAt = now
	m.byID[a.ID] = a
	m.seq++
	m.order[a.ID] = m.seq
	return a, nil
}

func (m *MemoryAddresses) ListByUser(_ context.Context, userID string) ([]Address, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []Address
	for _, a := range m.byID {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	// Default first, then newest first — matching the SQL ORDER BY.
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDefault != out[j].IsDefault {
			return out[i].IsDefault
		}
		return m.order[out[i].ID] > m.order[out[j].ID]
	})
	return out, nil
}

func (m *MemoryAddresses) Get(_ context.Context, userID, id string) (Address, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	a, ok := m.byID[id]
	if !ok || a.UserID != userID {
		return Address{}, ErrAddressNotFound
	}
	return a, nil
}

func (m *MemoryAddresses) Update(_ context.Context, a Address) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cur, ok := m.byID[a.ID]
	if !ok || cur.UserID != a.UserID {
		return ErrAddressNotFound
	}
	// Replace mutable fields only; id/user/default/created_at are preserved.
	cur.Label = a.Label
	cur.Recipient = a.Recipient
	cur.Phone = a.Phone
	cur.Line1 = a.Line1
	cur.Line2 = a.Line2
	cur.City = a.City
	cur.State = a.State
	cur.PostalCode = a.PostalCode
	cur.Country = a.Country
	cur.UpdatedAt = time.Now()
	m.byID[a.ID] = cur
	return nil
}

func (m *MemoryAddresses) Delete(_ context.Context, userID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.byID[id]
	if !ok || a.UserID != userID {
		return ErrAddressNotFound
	}
	delete(m.byID, id)
	delete(m.order, id)
	return nil
}

func (m *MemoryAddresses) SetDefault(_ context.Context, userID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.byID[id]
	if !ok || a.UserID != userID {
		return ErrAddressNotFound
	}
	m.clearDefault(userID)
	a.IsDefault = true
	a.UpdatedAt = time.Now()
	m.byID[id] = a
	return nil
}

// clearDefault unsets the default flag on all of the user's addresses. Caller
// holds the write lock.
func (m *MemoryAddresses) clearDefault(userID string) {
	for id, a := range m.byID {
		if a.UserID == userID && a.IsDefault {
			a.IsDefault = false
			m.byID[id] = a
		}
	}
}
