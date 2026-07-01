package notify

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/menawar/ecommerce-platform/pkg/events"
	"github.com/menawar/ecommerce-platform/services/notification/internal/db"
)

// topicTemplates maps event topics to notification templates. Topics not listed
// are simply ignored (acked, never turned into a notification).
var topicTemplates = map[string]string{
	"user.registered":               "welcome",
	"user.verification_requested":   "email_verification",
	"user.password_reset_requested": "password_reset",
	"order.paid":                    "payment_received",
	"order.confirmed":               "order_confirmation",
	"order.cancelled":               "order_cancelled",
	"order.shipped":                 "order_shipped",
	"order.delivered":               "order_delivered",
}

type Handler struct {
	q      *db.Queries
	sender Sender
	log    *slog.Logger
}

func NewHandler(pool *pgxpool.Pool, sender Sender, log *slog.Logger) *Handler {
	return &Handler{q: db.New(pool), sender: sender, log: log}
}

// Handle processes one event IDEMPOTENTLY. It returns nil on success or on a
// duplicate (caller should ack), and a non-nil error only on a transient failure
// the caller should retry (nak). Because delivery is at-least-once, the same event
// can arrive more than once; the UNIQUE event_id makes the second one a no-op.
func (h *Handler) Handle(ctx context.Context, env events.Envelope) error {
	template, ok := topicTemplates[env.Topic]
	if !ok {
		return nil // not a topic we notify on
	}

	eventID, err := uuid.Parse(env.EventID)
	if err != nil {
		h.log.WarnContext(ctx, "dropping event with bad event_id", "event_id", env.EventID)
		return nil // unparseable id -> drop (ack), retrying won't help
	}

	// Every payload we handle carries user_id; transactional-link emails (verify,
	// password reset) also carry a generic action_url. Extract best-effort.
	var data struct {
		UserID    string `json:"user_id"`
		ActionURL string `json:"action_url"`
		// verify_url is the pre-rename field name; read it as a fallback so events
		// already in the stream during a rolling deploy still carry their link.
		// TODO: remove one release after the action_url rename ships.
		VerifyURL string `json:"verify_url"`
	}
	_ = json.Unmarshal(env.Data, &data)
	if data.ActionURL == "" {
		data.ActionURL = data.VerifyURL
	}
	var userID pgtype.UUID
	if uid, perr := uuid.Parse(data.UserID); perr == nil {
		userID = pgtype.UUID{Bytes: uid, Valid: true}
	}

	// The dedup gate: insert keyed by the UNIQUE event_id. A duplicate delivery
	// fails here, and we treat that as "already handled" — the heart of an
	// idempotent consumer.
	err = h.q.InsertNotification(ctx, db.InsertNotificationParams{
		EventID:  pgtype.UUID{Bytes: eventID, Valid: true},
		UserID:   userID,
		Channel:  "email",
		Template: template,
		Payload:  env.Data,
	})
	if err != nil {
		if isUniqueViolation(err) {
			h.log.InfoContext(ctx, "duplicate event ignored", "event_id", env.EventID, "topic", env.Topic)
			return nil // already processed
		}
		return err // transient (e.g. DB down) -> nak/retry
	}

	// Deliver (mock logs). Note: we record-then-send, so the dedup is on the record;
	// a real sender that can fail would also need send-side idempotency — out of v1
	// scope (the mock never fails).
	return h.sender.Send(ctx, Notification{
		EventID:  env.EventID,
		UserID:   data.UserID,
		Channel:  "email",
		Template: template,
		Link:     data.ActionURL,
	})
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
