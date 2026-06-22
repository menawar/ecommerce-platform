// Package order holds the order domain: the status state machine and its rules.
// It has NO I/O — pure logic the saga consults to decide and guard transitions.
package order

// Status is an order's lifecycle state. The saga drives an order through these.
type Status string

const (
	StatusPending        Status = "PENDING"         // created, nothing reserved yet
	StatusStockReserved  Status = "STOCK_RESERVED"  // Product.ReserveStock succeeded
	StatusPaymentPending Status = "PAYMENT_PENDING"  // charge in flight
	StatusPaid           Status = "PAID"            // charge succeeded, stock committed
	StatusConfirmed      Status = "CONFIRMED"       // terminal success
	StatusPaymentFailed  Status = "PAYMENT_FAILED"  // charge declined
	StatusCancelled      Status = "CANCELLED"       // terminal failure (compensated)
)

// transitions is the allowed-next-states table. Encoding the state machine as
// data (not scattered if-statements) makes the rules reviewable in one place and
// the guard trivial. Terminal states map to nothing.
var transitions = map[Status][]Status{
	StatusPending:        {StatusStockReserved, StatusCancelled},
	StatusStockReserved:  {StatusPaymentPending, StatusCancelled},
	StatusPaymentPending: {StatusPaid, StatusPaymentFailed},
	StatusPaid:           {StatusConfirmed},
	StatusPaymentFailed:  {StatusCancelled},
	StatusConfirmed:      {}, // terminal
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

// IsTerminal reports whether the order has reached an end state (no further saga
// work). Used to short-circuit and to gate CancelOrder.
func (s Status) IsTerminal() bool {
	return s == StatusConfirmed || s == StatusCancelled
}
