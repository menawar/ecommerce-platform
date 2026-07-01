-- Migration 000005 (down): drop fulfillment columns.
ALTER TABLE orders
    DROP COLUMN IF EXISTS tracking_number,
    DROP COLUMN IF EXISTS shipped_at,
    DROP COLUMN IF EXISTS delivered_at;
