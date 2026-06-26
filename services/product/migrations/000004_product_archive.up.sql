-- Migration 000004 (up): soft-delete support for products.
--
-- Products can't be hard-deleted once ordered (reservation_items references them),
-- and orders in another service snapshot them anyway. archived_at NULL = active;
-- non-NULL = removed from the catalog but retained for history. List/Get filter on
-- it, so an archived product vanishes from the storefront and can't be sold.
ALTER TABLE products ADD COLUMN archived_at TIMESTAMPTZ;
