// Package provider abstracts the actual money movement. The Provider interface is
// the key seam: the service is written against it, so a real Stripe adapter can
// drop in later, and — crucially — the MockProvider makes the saga's payment
// FAILURE path deterministically testable without touching real money.
package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// ErrDeclined is a normal business outcome (the charge was refused), not a fault.
// The saga compensates on it (release stock, cancel order).
var ErrDeclined = errors.New("payment: declined by provider")

// Name constants stored on the payment row.
const (
	NameMock     = "mock"
	NamePaystack = "paystack"
)

// Verification statuses returned by AsyncProvider.Verify and stored on the payment
// row. They match the strings the payment server already uses.
const (
	StatusPending   = "pending"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

// Provider charges an amount and returns the provider's reference for the charge.
// ref is a caller-supplied correlation id (we pass the order id).
//
// This is the SYNCHRONOUS model: the result is known when Charge returns. It fits
// the MockProvider but not a real redirect-based PSP — see AsyncProvider, which
// the service is migrating to.
type Provider interface {
	Charge(ctx context.Context, amountCents int64, currency, ref string) (providerRef string, err error)
}

// AsyncProvider is the redirect+webhook model that real PSPs (Paystack,
// Flutterwave, …) use: Initialize creates a transaction the customer authorizes
// out-of-band, so the outcome is NOT known when it returns. The result arrives
// later — pushed via webhook, or pulled with Verify (the authoritative check we
// run before acting on any webhook).
type AsyncProvider interface {
	// Initialize starts a transaction and returns the URL to send the customer to
	// plus the provider's reference (persisted so a later webhook/Verify can find
	// this payment). amountCents is in the currency's minor unit (kobo for NGN).
	Initialize(ctx context.Context, amountCents int64, currency, ref, email string) (authorizationURL, providerRef string, err error)
	// Verify returns the current status of a transaction by its provider reference:
	// StatusSucceeded, StatusFailed, or StatusPending.
	Verify(ctx context.Context, providerRef string) (statusValue string, err error)
}

// Mock is a deterministic provider for local/dev and tests. It implements BOTH
// Provider (legacy sync) and AsyncProvider (the new model) so the service can
// migrate one step at a time while staying runnable offline.
type Mock struct{}

func NewMock() Mock { return Mock{} }

// Compile-time checks: Mock bridges both models.
var (
	_ Provider      = Mock{}
	_ AsyncProvider = Mock{}
)

// Charge succeeds for every amount EXCEPT those where amountCents % 100 == 13
// (e.g. a total ending in .13). That one rule lets a test — or the UI — force a
// decline on demand and exercise the compensation path, with zero randomness.
func (Mock) Charge(_ context.Context, amountCents int64, _ string, _ string) (string, error) {
	if amountCents%100 == 13 {
		return "", ErrDeclined
	}
	return "mock_" + uuid.NewString(), nil
}

// mockRefPrefix tags references Mock.Initialize mints. The amount is encoded into
// the reference so Verify can re-apply the deterministic %100==13 decline rule
// with nothing but the reference to go on — mirroring how a real webhook/Verify
// carries no order context, only the provider's id.
const mockRefPrefix = "mock_pi_"

// Initialize always "creates" the transaction (the customer hasn't paid yet, so
// there's nothing to decline here). It returns a fake authorization URL and a
// reference with the amount baked in for Verify to read back.
func (Mock) Initialize(_ context.Context, amountCents int64, _, _, _ string) (string, string, error) {
	ref := fmt.Sprintf("%s%d_%s", mockRefPrefix, amountCents, uuid.NewString())
	return "https://mock-psp.local/checkout/" + ref, ref, nil
}

// Verify decodes the amount from the reference and applies the same %100==13 rule
// as Charge, so the async decline path is just as deterministic as the sync one.
func (Mock) Verify(_ context.Context, providerRef string) (string, error) {
	rest, ok := strings.CutPrefix(providerRef, mockRefPrefix)
	if !ok {
		return "", fmt.Errorf("mock verify: unrecognized reference %q", providerRef)
	}
	amountPart, _, _ := strings.Cut(rest, "_")
	amountCents, err := strconv.ParseInt(amountPart, 10, 64)
	if err != nil {
		return "", fmt.Errorf("mock verify: bad amount in reference %q: %w", providerRef, err)
	}
	if amountCents%100 == 13 {
		return StatusFailed, nil
	}
	return StatusSucceeded, nil
}
