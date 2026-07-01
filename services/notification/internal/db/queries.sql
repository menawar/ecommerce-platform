-- name: ClaimNotificationForSend :one
-- Claim an event for delivery, idempotently. First delivery inserts a pending row;
-- a redelivery of a not-yet-terminal event returns it so the caller retries. A
-- redelivery of an event already in a TERMINAL state (sent, or failed/dead-lettered)
-- matches the WHERE-false branch, updates nothing, and returns no row (ErrNoRows) —
-- the caller treats that as "already handled" and skips (idempotent, no reprocess or
-- double dead-letter). This is the dedup gate folded into the retry ledger. attempts
-- is NOT bumped here: it counts SEND failures (see RecordNotificationError),
-- decoupled from JetStream delivery count.
INSERT INTO notifications (event_id, user_id, channel, template, payload, status)
VALUES ($1, $2, $3, $4, $5, 'pending')
ON CONFLICT (event_id) DO UPDATE
    SET updated_at = now()
    WHERE notifications.status NOT IN ('sent', 'failed')
RETURNING event_id;

-- name: MarkNotificationSent :exec
UPDATE notifications SET status = 'sent', sent_at = now(), last_error = NULL, updated_at = now()
WHERE event_id = $1;

-- name: RecordNotificationError :one
-- A failed SEND that will be retried: bump the attempt count, record the error,
-- leave status pending. Returns the new attempt count so the caller can decide
-- whether to dead-letter.
UPDATE notifications SET attempts = attempts + 1, last_error = $2, updated_at = now()
WHERE event_id = $1
RETURNING attempts;

-- name: MarkNotificationFailed :exec
-- Dead-letter: attempts exhausted, stop retrying.
UPDATE notifications SET status = 'failed', last_error = $2, updated_at = now()
WHERE event_id = $1;

-- name: CountByEventID :one
SELECT count(*) FROM notifications WHERE event_id = $1;

-- name: ListByUser :many
SELECT * FROM notifications WHERE user_id = $1 ORDER BY COALESCE(sent_at, updated_at) DESC LIMIT $2;
