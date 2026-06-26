-- Migration 000002 (up): transactional outbox for paymentdb.
-- The payment service is moving to the async PSP model (Paystack): when a webhook
-- is verified, the status transition and its domain event (payment.succeeded /
-- payment.failed) must be written atomically. This table is the outbox a poller
-- drains to NATS — same pattern as the order service.
CREATE TABLE outbox (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic        TEXT NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

-- Partial index: the poller only ever scans UNPUBLISHED rows, so index just those.
CREATE INDEX idx_payment_outbox_unpublished ON outbox(created_at) WHERE published_at IS NULL;

-- The webhook looks a payment up by the provider's reference, so index it.
CREATE INDEX idx_payments_provider_ref ON payments(provider_ref);
