// Package provider abstracts the actual money movement. The Provider interface is
// the key seam: the service is written against it, so a real Stripe adapter can
// drop in later, and — crucially — the MockProvider makes the saga's payment
// FAILURE path deterministically testable without touching real money.
package provider

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrDeclined is a normal business outcome (the charge was refused), not a fault.
// The saga compensates on it (release stock, cancel order).
var ErrDeclined = errors.New("payment: declined by provider")

// Name constants stored on the payment row.
const (
	NameMock = "mock"
)

// Provider charges an amount and returns the provider's reference for the charge.
// ref is a caller-supplied correlation id (we pass the order id).
type Provider interface {
	Charge(ctx context.Context, amountCents int64, currency, ref string) (providerRef string, err error)
}

// Mock is a deterministic provider for local/dev and tests.
type Mock struct{}

func NewMock() Mock { return Mock{} }

// Compile-time check.
var _ Provider = Mock{}

// Charge succeeds for every amount EXCEPT those where amountCents % 100 == 13
// (e.g. a total ending in .13). That one rule lets a test — or the UI — force a
// decline on demand and exercise the compensation path, with zero randomness.
func (Mock) Charge(_ context.Context, amountCents int64, _ string, _ string) (string, error) {
	if amountCents%100 == 13 {
		return "", ErrDeclined
	}
	return "mock_" + uuid.NewString(), nil
}
