-- Migration 000006 (up): support account erasure (NDPR/GDPR). deleted_at marks an
-- anonymised (tombstoned) account so we can make deletion idempotent and reject
-- re-activation.
ALTER TABLE users ADD COLUMN deleted_at TIMESTAMPTZ;
