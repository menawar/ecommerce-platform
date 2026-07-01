-- Migration 000005 (up): fulfillment. An admin ships a CONFIRMED order (optionally
-- with a tracking number), then marks it delivered. Timestamps are nullable — set
-- when each step happens.
ALTER TABLE orders
    ADD COLUMN tracking_number TEXT NOT NULL DEFAULT '',
    ADD COLUMN shipped_at      TIMESTAMPTZ,
    ADD COLUMN delivered_at    TIMESTAMPTZ;
