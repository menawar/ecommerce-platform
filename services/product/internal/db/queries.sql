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
