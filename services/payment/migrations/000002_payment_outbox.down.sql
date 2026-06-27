-- Migration 000002 (down): drop the payment outbox and provider_ref index.
DROP INDEX IF EXISTS idx_payments_provider_ref;
DROP INDEX IF EXISTS idx_payment_outbox_unpublished;
DROP TABLE IF EXISTS outbox;
