-- Migration 000005 (up): saved shipping addresses. A user keeps an address book;
-- each order snapshots the chosen one (the order service owns that copy), so later
-- edits here never rewrite past orders. At most one address per user is the default.
CREATE TABLE addresses (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL,
    label       TEXT NOT NULL DEFAULT '',   -- "Home", "Work", optional
    recipient   TEXT NOT NULL,              -- who receives it (may differ from account)
    phone       TEXT NOT NULL,
    line1       TEXT NOT NULL,
    line2       TEXT NOT NULL DEFAULT '',
    city        TEXT NOT NULL,
    state       TEXT NOT NULL,
    postal_code TEXT NOT NULL DEFAULT '',
    country     TEXT NOT NULL DEFAULT 'NG',
    is_default  BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_addresses_user ON addresses(user_id, created_at DESC);
