-- Migration 000001 (up): payments table for paymentdb.
CREATE TABLE payments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID NOT NULL,
    amount_cents    BIGINT NOT NULL CHECK (amount_cents >= 0),
    currency        TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed')),
    provider        TEXT NOT NULL,
    provider_ref    TEXT,
    -- The idempotency key dedupes "create payment" requests: a UNIQUE constraint
    -- turns a retried charge into a no-op (the second insert fails, we return the
    -- first). NOT NULL — every payment must be idempotent.
    idempotency_key TEXT UNIQUE NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
