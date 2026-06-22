package order_test

import (
	"testing"

	"github.com/menawar/ecommerce-platform/services/order/internal/order"
)

func TestTransitions_Allowed(t *testing.T) {
	ok := []struct{ from, to order.Status }{
		{order.StatusPending, order.StatusStockReserved},
		{order.StatusPending, order.StatusCancelled},
		{order.StatusStockReserved, order.StatusPaymentPending},
		{order.StatusStockReserved, order.StatusCancelled},
		{order.StatusPaymentPending, order.StatusPaid},
		{order.StatusPaymentPending, order.StatusPaymentFailed},
		{order.StatusPaid, order.StatusConfirmed},
		{order.StatusPaymentFailed, order.StatusCancelled},
	}
	for _, tc := range ok {
		if !tc.from.CanTransitionTo(tc.to) {
			t.Errorf("%s -> %s should be allowed", tc.from, tc.to)
		}
	}
}

func TestTransitions_Rejected(t *testing.T) {
	bad := []struct{ from, to order.Status }{
		{order.StatusPending, order.StatusPaid},        // can't skip reservation/payment
		{order.StatusCancelled, order.StatusPaid},      // terminal can't move
		{order.StatusConfirmed, order.StatusCancelled}, // terminal can't move
		{order.StatusPaid, order.StatusPaymentFailed},  // already paid
	}
	for _, tc := range bad {
		if tc.from.CanTransitionTo(tc.to) {
			t.Errorf("%s -> %s should be rejected", tc.from, tc.to)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	terminal := []order.Status{order.StatusConfirmed, order.StatusCancelled}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
	nonTerminal := []order.Status{order.StatusPending, order.StatusStockReserved, order.StatusPaymentPending, order.StatusPaid, order.StatusPaymentFailed}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("%s should not be terminal", s)
		}
	}
}
