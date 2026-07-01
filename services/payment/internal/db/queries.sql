-- name: GetPaymentByID :one
SELECT * FROM payments WHERE id = $1;

-- name: GetPaymentByIdempotencyKey :one
SELECT * FROM payments WHERE idempotency_key = $1;

-- name: CreatePayment :one
INSERT INTO payments (order_id, amount_cents, currency, status, provider, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdatePaymentResult :one
UPDATE payments
SET status = $2, provider_ref = $3
WHERE id = $1
RETURNING *;

-- name: GetPaymentByProviderRef :one
SELECT * FROM payments WHERE provider_ref = $1;

-- MarkPaymentRefunded flips a succeeded charge to refunded. The status precondition
-- makes it an atomic compare-and-set: only a currently-succeeded payment refunds,
-- and a concurrent second refund matches 0 rows.
-- name: MarkPaymentRefunded :one
UPDATE payments SET status = 'refunded' WHERE id = $1 AND status = 'succeeded' RETURNING *;

-- Outbox: events written in the same tx as a payment status change, drained to
-- NATS by the shared poller (see pkg/outbox).

-- name: InsertOutbox :exec
INSERT INTO outbox (topic, payload) VALUES ($1, $2);

-- name: ListUnpublishedOutbox :many
SELECT * FROM outbox WHERE published_at IS NULL ORDER BY created_at LIMIT $1;

-- name: MarkOutboxPublished :exec
UPDATE outbox SET published_at = now() WHERE id = $1;
