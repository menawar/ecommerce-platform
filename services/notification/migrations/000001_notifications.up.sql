-- Migration 000001 (up): notificationdb.
CREATE TABLE notifications (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- event_id is UNIQUE: it's the idempotency key. Because outbox/NATS delivery is
    -- AT-LEAST-ONCE, the same event can arrive twice; inserting with a unique
    -- event_id turns a duplicate into a no-op (the second insert fails). This is the
    -- "store processed event_ids and skip duplicates" rule, folded into one table.
    event_id  UUID UNIQUE NOT NULL,
    user_id   UUID,
    channel   TEXT NOT NULL,    -- email | sms
    template  TEXT NOT NULL,    -- welcome, order_confirmation, ...
    payload   JSONB NOT NULL,
    sent_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user ON notifications(user_id, sent_at DESC);
