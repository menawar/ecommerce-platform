-- name: CreateOrder :one
-- id is supplied (not defaulted) so reservation_id can equal the order id — the
-- saga reserves stock under the order's own id, making ReserveStock idempotent.
-- total_cents already includes shipping_cents; the address + method name are
-- snapshotted so later edits never rewrite the order.
INSERT INTO orders (
    id, user_id, status, total_cents, currency, reservation_id, idempotency_key,
    shipping_method_id, shipping_method_name, shipping_cents,
    ship_recipient, ship_phone, ship_line1, ship_line2, ship_city, ship_state, ship_postal_code, ship_country
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
RETURNING *;

-- name: GetOrder :one
SELECT * FROM orders WHERE id = $1;

-- name: GetOrderByIdempotencyKey :one
SELECT * FROM orders WHERE idempotency_key = $1;

-- name: ListOrdersByUser :many
SELECT * FROM orders WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3;

-- name: ListAllOrders :many
-- Admin view: every order, newest first (the Gateway enforces the admin role).
SELECT * FROM orders ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- MarkOrderShipped/Delivered are atomic compare-and-set: the status precondition in
-- WHERE means two concurrent callers can't both win — the loser matches 0 rows
-- (pgx.ErrNoRows), so only one order.shipped/delivered event is ever written.
-- name: MarkOrderShipped :one
UPDATE orders
SET status = 'SHIPPED', tracking_number = $2, shipped_at = now(), updated_at = now()
WHERE id = $1 AND status = 'CONFIRMED'
RETURNING *;

-- name: MarkOrderDelivered :one
UPDATE orders
SET status = 'DELIVERED', delivered_at = now(), updated_at = now()
WHERE id = $1 AND status = 'SHIPPED'
RETURNING *;

-- name: UpdateOrderStatus :one
UPDATE orders SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: SetOrderPaymentAndStatus :one
UPDATE orders SET payment_id = $2, status = $3, updated_at = now() WHERE id = $1 RETURNING *;

-- name: MarkOrderPaymentPending :one
-- The async "start" half records the initialized payment and its authorization URL
-- as the order enters PAYMENT_PENDING, awaiting the webhook-driven resume.
UPDATE orders
SET status = 'PAYMENT_PENDING', payment_id = $2, authorization_url = $3, updated_at = now()
WHERE id = $1
RETURNING *;

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

-- Shipping methods.

-- name: CreateShippingMethod :one
INSERT INTO shipping_methods (name, description, price_cents, sort_order, active)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetShippingMethod :one
SELECT * FROM shipping_methods WHERE id = $1;

-- name: ListShippingMethods :many
SELECT * FROM shipping_methods ORDER BY sort_order, name;

-- name: ListActiveShippingMethods :many
SELECT * FROM shipping_methods WHERE active ORDER BY sort_order, name;

-- name: UpdateShippingMethod :one
UPDATE shipping_methods
SET name = $2, description = $3, price_cents = $4, sort_order = $5, active = $6, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteShippingMethod :execrows
DELETE FROM shipping_methods WHERE id = $1;
