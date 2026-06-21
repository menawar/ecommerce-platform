-- Migration 000001 (down): exact inverse of the up migration, dropped in reverse
-- dependency order (inventory and the FK-referencing tables before the tables
-- they reference). A correct down migration is what makes a rollback safe.

DROP INDEX IF EXISTS idx_products_category;
DROP TABLE IF EXISTS inventory;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS categories;
