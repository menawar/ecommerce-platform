// Package store defines how the cart is persisted and provides a Redis-backed
// implementation. The cart is intentionally tiny: a set of (product_id, quantity)
// lines and nothing more.
package store

import "context"

// Item is one cart line. Notice what's NOT here: price, name, currency. The cart
// stores only WHAT and HOW MANY — never money. Prices are authoritative only at
// checkout, where the Order service asks the Product service for the current
// price. If we stored a price in the cart (or trusted one from the client), a user
// could tamper with it and pay whatever they wanted. This is the trust boundary.
type Item struct {
	ProductID string
	Quantity  int32
}

// Store is the cart persistence PORT. The service depends on this interface, not on
// Redis directly, so tests can run against an in-memory fake (miniredis) and the
// implementation can change without touching the gRPC handlers — accept interfaces,
// return structs.
//
// Every method is keyed by userID: a cart belongs to exactly one user, and that
// id comes from the validated JWT at the gateway, never from client input.
type Store interface {
	Get(ctx context.Context, userID string) ([]Item, error)
	// AddItem increments product's quantity by delta (the qty being added) and
	// returns the resulting cart. A delta that drives the total to <= 0 removes the line.
	AddItem(ctx context.Context, userID, productID string, delta int32) ([]Item, error)
	// SetItem sets the absolute quantity (used by "update quantity"). qty <= 0 removes it.
	SetItem(ctx context.Context, userID, productID string, qty int32) ([]Item, error)
	RemoveItem(ctx context.Context, userID, productID string) ([]Item, error)
	Clear(ctx context.Context, userID string) error
}
