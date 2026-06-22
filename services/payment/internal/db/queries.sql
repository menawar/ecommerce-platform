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
