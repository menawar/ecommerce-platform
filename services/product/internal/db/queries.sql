-- Catalog read/write queries. Each is annotated for sqlc:
--   :one  -> returns exactly one row  (Go: (T, error))
--   :many -> returns many rows         (Go: ([]T, error))
--   :exec -> returns no rows           (Go: error)
-- $1, $2, ... are positional params; sqlc.narg(name) is a NULLABLE named param.

-- name: CreateCategory :one
INSERT INTO categories (name, slug)
VALUES ($1, $2)
RETURNING *;

-- name: CreateProduct :one
INSERT INTO products (sku, name, description, price_cents, currency, category_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetProduct :one
SELECT * FROM products
WHERE id = $1;

-- name: ListProducts :many
-- Optional category filter via a nullable named arg: when category_id is NULL the
-- first branch short-circuits and every row matches.
SELECT * FROM products
WHERE sqlc.narg('category_id')::uuid IS NULL
   OR category_id = sqlc.narg('category_id')
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountProducts :one
-- Total for the same filter, so ListProducts can report total alongside the page.
SELECT count(*) FROM products
WHERE sqlc.narg('category_id')::uuid IS NULL
   OR category_id = sqlc.narg('category_id');

-- name: CreateInventory :one
INSERT INTO inventory (product_id, quantity)
VALUES ($1, $2)
RETURNING *;

-- name: GetInventory :one
SELECT * FROM inventory
WHERE product_id = $1;

-- name: GetProductWithInventory :one
-- Product detail joined with live stock; available = quantity - reserved.
SELECT p.*, i.quantity, i.reserved, (i.quantity - i.reserved)::int AS available
FROM products p
JOIN inventory i ON i.product_id = p.id
WHERE p.id = $1;

-- name: ListProductsWithInventory :many
SELECT p.*, i.quantity, i.reserved, (i.quantity - i.reserved)::int AS available
FROM products p
JOIN inventory i ON i.product_id = p.id
WHERE sqlc.narg('category_id')::uuid IS NULL
   OR p.category_id = sqlc.narg('category_id')
ORDER BY p.created_at DESC
LIMIT $1 OFFSET $2;

-- name: InsertReservation :exec
-- The idempotency gate: the PK on id rejects a duplicate reservation_id.
INSERT INTO stock_reservations (id) VALUES ($1);

-- name: ReserveInventory :execrows
-- The oversell guard, as ONE atomic statement. Returns rows-affected: 1 if the
-- row had enough available stock (quantity - reserved >= qty), 0 otherwise. The
-- row is locked for the duration, so concurrent reservers serialize and can never
-- both pass the check on the same stock. version is bumped as a change marker.
UPDATE inventory
SET reserved = reserved + sqlc.arg('quantity'),
    version  = version + 1
WHERE product_id = sqlc.arg('product_id')
  AND quantity - reserved >= sqlc.arg('quantity');

-- name: InsertReservationItem :exec
INSERT INTO reservation_items (reservation_id, product_id, quantity)
VALUES ($1, $2, $3);
