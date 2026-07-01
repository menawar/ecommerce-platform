// Package order holds the order domain: the status state machine and its rules.
// It has NO I/O — pure logic the saga consults to decide and guard transitions.
package order

// Status is an order's lifecycle state. The saga drives an order through these.
type Status string

const (
	StatusPending        Status = "PENDING"        // created, nothing reserved yet
	StatusStockReserved  Status = "STOCK_RESERVED" // Product.ReserveStock succeeded
	StatusPaymentPending Status = "PAYMENT_PENDING" // charge in flight
	StatusPaid           Status = "PAID"           // charge succeeded, stock committed
	StatusConfirmed      Status = "CONFIRMED"      // paid + confirmed, awaiting fulfillment
	StatusShipped        Status = "SHIPPED"        // handed to delivery (has tracking)
	StatusDelivered      Status = "DELIVERED"      // fulfilled (still refundable — returns)
	StatusRefunded       Status = "REFUNDED"       // terminal: charge reversed
	StatusPaymentFailed  Status = "PAYMENT_FAILED" // charge declined
	StatusCancelled      Status = "CANCELLED"      // terminal failure (compensated)
)

// transitions is the allowed-next-states table. Encoding the state machine as
// data (not scattered if-statements) makes the rules reviewable in one place and
// the guard trivial. Terminal states map to nothing.
var transitions = map[Status][]Status{
	StatusPending:        {StatusStockReserved, StatusCancelled},
	StatusStockReserved:  {StatusPaymentPending, StatusCancelled},
	StatusPaymentPending: {StatusPaid, StatusPaymentFailed},
	// Money is captured from PAID onward, so every such state is refundable — incl.
	// a PAID order whose confirm step stalled, and one already delivered (returns).
	StatusPaid:           {StatusConfirmed, StatusRefunded},
	StatusConfirmed:      {StatusShipped, StatusRefunded},
	StatusShipped:        {StatusDelivered, StatusRefunded},
	StatusDelivered:      {StatusRefunded},
	StatusRefunded:       {}, // terminal
	StatusPaymentFailed:  {StatusCancelled},
	StatusCancelled:      {}, // terminal
}

// CanTransitionTo reports whether moving from s to next is a legal step. The saga
// checks this before every state change so a bug can't, say, jump a CANCELLED
// order back to PAID.
func (s Status) CanTransitionTo(next Status) bool {
	for _, allowed := range transitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// IsTerminal reports whether the order has reached an end state (no further work).
func (s Status) IsTerminal() bool {
	return s == StatusRefunded || s == StatusCancelled
}

// IsPostPayment reports whether the order's payment outcome is already settled —
// i.e. it has reached CONFIRMED (or beyond) or been CANCELLED. The payment-resume
// consumer short-circuits on these (a late/duplicate payment event is a no-op),
// and CancelOrder refuses them (a paid order can only be refunded, never cancelled).
func (s Status) IsPostPayment() bool {
	switch s {
	case StatusConfirmed, StatusShipped, StatusDelivered, StatusRefunded, StatusCancelled:
		return true
	default:
		return false
	}
}
