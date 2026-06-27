// Package provider abstracts the actual money movement. The Provider interface is
// the key seam: the service is written against it, so a real Stripe adapter can
// drop in later, and — crucially — the MockProvider makes the saga's payment
// FAILURE path deterministically testable without touching real money.
package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

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

// Mock is a deterministic AsyncProvider for local/dev and tests — it lets the saga
// run and the decline path stay testable, offline, without real money.
type Mock struct{}

func NewMock() Mock { return Mock{} }

// Compile-time check.
var _ AsyncProvider = Mock{}

// mockRefPrefix tags references Mock.Initialize mints. The amount is encoded into
// the reference so Verify can re-apply the deterministic %100==13 decline rule
// with nothing but the reference to go on — mirroring how a real webhook/Verify
// carries no order context, only the provider's id.
const mockRefPrefix = "mock_pi_"

// Initialize always "creates" the transaction (the customer hasn't paid yet, so
// there's nothing to decline here). It returns a RELATIVE authorization URL — an
// in-app dev simulator page — and a reference with the amount baked in for Verify
// to read back. A real PSP returns an absolute https URL; the web BFF distinguishes
// the two (relative => internal mock page) when it redirects the customer.
func (Mock) Initialize(_ context.Context, amountCents int64, _, _, _ string) (string, string, error) {
	ref := fmt.Sprintf("%s%d_%s", mockRefPrefix, amountCents, uuid.NewString())
	return "/checkout/pay/" + ref, ref, nil
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
