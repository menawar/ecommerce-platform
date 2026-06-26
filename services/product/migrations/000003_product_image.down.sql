-- Migration 000003 (down): drop the product image URL column.
ALTER TABLE products DROP COLUMN image_url;
