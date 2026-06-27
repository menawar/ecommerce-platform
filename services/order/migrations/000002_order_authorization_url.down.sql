-- Migration 000002 (down): drop the authorization_url column.
ALTER TABLE orders DROP COLUMN IF EXISTS authorization_url;
