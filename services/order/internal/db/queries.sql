-- name: CreateOrder :one
INSERT INTO orders (user_id, status, total_cents, currency, reservation_id, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetOrder :one
SELECT * FROM orders WHERE id = $1;

-- name: GetOrderByIdempotencyKey :one
SELECT * FROM orders WHERE idempotency_key = $1;

-- name: ListOrdersByUser :many
SELECT * FROM orders WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: UpdateOrderStatus :one
UPDATE orders SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: SetOrderPaymentAndStatus :one
UPDATE orders SET payment_id = $2, status = $3, updated_at = now() WHERE id = $1 RETURNING *;

-- name: CreateOrderItem :exec
INSERT INTO order_items (order_id, product_id, name, price_cents, quantity)
VALUES ($1, $2, $3, $4, $5);

-- name: ListOrderItems :many
SELECT * FROM order_items WHERE order_id = $1 ORDER BY name;

-- name: InsertOutbox :exec
INSERT INTO outbox (topic, payload) VALUES ($1, $2);

-- name: ListUnpublishedOutbox :many
SELECT * FROM outbox WHERE published_at IS NULL ORDER BY created_at LIMIT $1;

-- name: MarkOutboxPublished :exec
UPDATE outbox SET published_at = now() WHERE id = $1;
