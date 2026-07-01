-- Migration 000002 (down): revert the delivery ledger.
DROP INDEX IF EXISTS idx_notifications_status;

-- Restore sent_at to its old NOT NULL DEFAULT now() shape (backfill nulls first).
UPDATE notifications SET sent_at = now() WHERE sent_at IS NULL;
ALTER TABLE notifications ALTER COLUMN sent_at SET DEFAULT now();
ALTER TABLE notifications ALTER COLUMN sent_at SET NOT NULL;

ALTER TABLE notifications
    DROP COLUMN status,
    DROP COLUMN attempts,
    DROP COLUMN last_error,
    DROP COLUMN updated_at;
