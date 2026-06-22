package store

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// cartTTL: a cart expires after this much inactivity. Redis evicts the key
// automatically, so abandoned carts don't accumulate forever — TTL is the cart's
// garbage collector. We refresh it on every write (a sliding window).
const cartTTL = 30 * 24 * time.Hour

// Redis stores each cart as a single Redis HASH at key "cart:<user_id>", with one
// field per product (field = product_id, value = quantity). A hash is the natural
// fit: O(1) per-field add/update/remove, and HGETALL reads the whole cart at once.
type Redis struct {
	client *redis.Client
}

// NewRedis returns the concrete *Redis (return structs); callers hold it as a Store.
func NewRedis(client *redis.Client) *Redis {
	return &Redis{client: client}
}

// Compile-time check that *Redis satisfies the Store interface.
var _ Store = (*Redis)(nil)

func cartKey(userID string) string { return "cart:" + userID }

func (r *Redis) Get(ctx context.Context, userID string) ([]Item, error) {
	vals, err := r.client.HGetAll(ctx, cartKey(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("cart: hgetall: %w", err)
	}
	return toItems(vals), nil
}

func (r *Redis) AddItem(ctx context.Context, userID, productID string, delta int32) ([]Item, error) {
	key := cartKey(userID)
	// HINCRBY is atomic server-side: two concurrent "add 1" requests for the same
	// product accumulate to +2, never lost — no read-modify-write race in our code.
	newQty, err := r.client.HIncrBy(ctx, key, productID, int64(delta)).Result()
	if err != nil {
		return nil, fmt.Errorf("cart: hincrby: %w", err)
	}
	if newQty <= 0 {
		// A net non-positive quantity means "not in the cart" — drop the field.
		if err := r.client.HDel(ctx, key, productID).Err(); err != nil {
			return nil, fmt.Errorf("cart: hdel: %w", err)
		}
	}
	if err := r.touchTTL(ctx, key); err != nil {
		return nil, err
	}
	return r.Get(ctx, userID)
}

func (r *Redis) SetItem(ctx context.Context, userID, productID string, qty int32) ([]Item, error) {
	if qty <= 0 {
		return r.RemoveItem(ctx, userID, productID)
	}
	key := cartKey(userID)
	if err := r.client.HSet(ctx, key, productID, qty).Err(); err != nil {
		return nil, fmt.Errorf("cart: hset: %w", err)
	}
	if err := r.touchTTL(ctx, key); err != nil {
		return nil, err
	}
	return r.Get(ctx, userID)
}

func (r *Redis) RemoveItem(ctx context.Context, userID, productID string) ([]Item, error) {
	if err := r.client.HDel(ctx, cartKey(userID), productID).Err(); err != nil {
		return nil, fmt.Errorf("cart: hdel: %w", err)
	}
	return r.Get(ctx, userID)
}

func (r *Redis) Clear(ctx context.Context, userID string) error {
	// DEL removes the whole hash (and its TTL). Used by the Order service after a
	// successful checkout.
	if err := r.client.Del(ctx, cartKey(userID)).Err(); err != nil {
		return fmt.Errorf("cart: del: %w", err)
	}
	return nil
}

// touchTTL refreshes the sliding expiry. Expire on a non-existent key is a no-op,
// which is fine (an empty cart simply has no key to keep alive).
func (r *Redis) touchTTL(ctx context.Context, key string) error {
	if err := r.client.Expire(ctx, key, cartTTL).Err(); err != nil {
		return fmt.Errorf("cart: expire: %w", err)
	}
	return nil
}

// toItems converts Redis's string map into typed, deterministically-ordered items.
// Redis hashes are unordered, so we sort by product_id for stable responses/tests.
func toItems(vals map[string]string) []Item {
	items := make([]Item, 0, len(vals))
	for productID, qtyStr := range vals {
		qty, _ := strconv.Atoi(qtyStr) // we only ever write integers
		items = append(items, Item{ProductID: productID, Quantity: int32(qty)})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ProductID < items[j].ProductID })
	return items
}
