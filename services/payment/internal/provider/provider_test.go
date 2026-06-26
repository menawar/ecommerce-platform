package provider_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/menawar/ecommerce-platform/services/payment/internal/provider"
)

// TestMockCharge proves the deterministic rule: amounts ending such that
// amount % 100 == 13 are declined; everything else succeeds with a ref.
func TestMockCharge(t *testing.T) {
	m := provider.NewMock()
	ctx := context.Background()

	tests := []struct {
		name       string
		amount     int64
		wantDecline bool
	}{
		{"round amount succeeds", 2500, false},
		{"non-13 succeeds", 1999, false},
		{"...13 declines", 1313, true},
		{"113 declines", 113, true},
		{"13 declines", 13, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := m.Charge(ctx, tc.amount, "NGN", "order-1")
			if tc.wantDecline {
				if !errors.Is(err, provider.ErrDeclined) {
					t.Errorf("amount %d: want ErrDeclined, got %v", tc.amount, err)
				}
				return
			}
			if err != nil {
				t.Errorf("amount %d: unexpected error %v", tc.amount, err)
			}
			if !strings.HasPrefix(ref, "mock_") {
				t.Errorf("ref %q, want mock_ prefix", ref)
			}
		})
	}
}

// TestMockAsync proves the async model is just as deterministic: Initialize never
// fails (the customer hasn't paid yet), and Verify re-derives the %100==13 decline
// from the reference alone — the only thing a real webhook/Verify would carry.
func TestMockAsync(t *testing.T) {
	m := provider.NewMock()
	ctx := context.Background()

	tests := []struct {
		name       string
		amount     int64
		wantStatus string
	}{
		{"round amount succeeds", 2500, provider.StatusSucceeded},
		{"non-13 succeeds", 1999, provider.StatusSucceeded},
		{"...13 fails", 1313, provider.StatusFailed},
		{"13 fails", 13, provider.StatusFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			url, ref, err := m.Initialize(ctx, tc.amount, "NGN", "order-1", "buyer@example.com")
			if err != nil {
				t.Fatalf("Initialize: unexpected error %v", err)
			}
			if url == "" || ref == "" {
				t.Fatalf("Initialize: empty url %q or ref %q", url, ref)
			}

			got, err := m.Verify(ctx, ref)
			if err != nil {
				t.Fatalf("Verify: unexpected error %v", err)
			}
			if got != tc.wantStatus {
				t.Errorf("Verify(amount %d) = %q, want %q", tc.amount, got, tc.wantStatus)
			}
		})
	}
}

// TestMockVerify_BadRef: a reference Mock didn't mint is an error, not a silent
// success — Verify must never invent a "succeeded" out of an unknown id.
func TestMockVerify_BadRef(t *testing.T) {
	if _, err := provider.NewMock().Verify(context.Background(), "not-a-mock-ref"); err == nil {
		t.Fatal("Verify: want error for unrecognized reference, got nil")
	}
}
