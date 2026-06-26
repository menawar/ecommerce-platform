-- Migration 000003 (up): add an optional product image URL.
--
-- NOT NULL DEFAULT '' so existing rows backfill to "no image" and any writer that
-- omits the column stays valid. The storefront treats '' as "render a placeholder".
-- This is just the catalog URL string; actual upload/object-storage lands later.
ALTER TABLE products ADD COLUMN image_url TEXT NOT NULL DEFAULT '';
