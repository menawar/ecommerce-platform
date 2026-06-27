-- Migration 000002 (up): the async payment flow returns a PSP hosted-checkout URL
-- the customer must visit to authorize payment. We persist it on the order so the
-- saga's "start" half can hand it back (and a retry can return the same one).
ALTER TABLE orders ADD COLUMN authorization_url TEXT NOT NULL DEFAULT '';
