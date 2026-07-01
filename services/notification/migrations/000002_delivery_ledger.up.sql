-- Migration 000002 (up): turn notifications into a delivery ledger so a failed
-- send is retried (not lost) and dead-lettered after a bounded number of attempts.
ALTER TABLE notifications
    ADD COLUMN status     TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'sent', 'failed')),
    ADD COLUMN attempts   INT NOT NULL DEFAULT 0,
    ADD COLUMN last_error TEXT,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- sent_at now means "actually delivered", so it's nullable and no longer defaulted.
ALTER TABLE notifications ALTER COLUMN sent_at DROP NOT NULL;
ALTER TABLE notifications ALTER COLUMN sent_at DROP DEFAULT;

-- Every pre-existing row was delivered under the old (never-failing LogSender) model.
UPDATE notifications SET status = 'sent' WHERE status = 'pending';

-- Index to find dead-lettered notifications for ops/alerting.
CREATE INDEX idx_notifications_status ON notifications(status) WHERE status = 'failed';
