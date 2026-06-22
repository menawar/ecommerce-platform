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
