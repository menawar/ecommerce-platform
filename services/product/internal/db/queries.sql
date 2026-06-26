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
INSERT INTO products (sku, name, description, price_cents, currency, category_id, image_url)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetProduct :one
SELECT * FROM products
WHERE id = $1;

-- name: CountProducts :one
-- Total matching the SAME filters as ListProductsWithInventory, so a page can
-- report the overall total. ILIKE is case-insensitive LIKE; the value is a bound
-- parameter (never string-concatenated), so it's injection-safe.
SELECT count(*) FROM products
WHERE (sqlc.narg('category_id')::uuid IS NULL OR category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('search')::text IS NULL OR name ILIKE '%' || sqlc.narg('search') || '%');

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
WHERE (sqlc.narg('category_id')::uuid IS NULL OR p.category_id = sqlc.narg('category_id'))
  AND (sqlc.narg('search')::text IS NULL OR p.name ILIKE '%' || sqlc.narg('search') || '%')
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

-- name: GetReservationStatusForUpdate :one
-- SELECT ... FOR UPDATE locks the reservation row for the rest of the transaction,
-- serializing concurrent Release/Commit of the same reservation so they can't both
-- act on it (idempotency under contention).
SELECT status FROM stock_reservations WHERE id = $1 FOR UPDATE;

-- name: SetReservationStatus :exec
UPDATE stock_reservations SET status = $2 WHERE id = $1;

-- name: ListReservationItems :many
SELECT product_id, quantity FROM reservation_items WHERE reservation_id = $1;

-- name: ReleaseInventory :exec
-- Compensation: give the held units back to "available" (drop reserved).
UPDATE inventory
SET reserved = reserved - sqlc.arg('quantity'),
    version  = version + 1
WHERE product_id = sqlc.arg('product_id');

-- name: CommitInventory :exec
-- Finalize: the goods actually leave — drop BOTH quantity and reserved.
UPDATE inventory
SET quantity = quantity - sqlc.arg('quantity'),
    reserved = reserved - sqlc.arg('quantity'),
    version  = version + 1
WHERE product_id = sqlc.arg('product_id');
