package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/menawar/ecommerce-platform/pkg/events"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/notification/internal/db"
)

// maxSendAttempts caps delivery retries; on the last attempt a failed send is
// dead-lettered (status='failed') instead of retried forever. Kept in step with the
// JetStream consumer's MaxDeliver so the app makes the terminal decision.
const maxSendAttempts = 5

// deadLettered counts notifications that exhausted their retries — an ops signal
// (alert on rate > 0) that email delivery is broken.
var deadLettered = promauto.NewCounter(prometheus.CounterOpts{
	Name: "notification_deadlettered_total",
	Help: "Notifications that failed to send after the maximum number of attempts.",
})

// userLookup is the sliver of the User service the notifier needs: resolve a
// recipient's email + name by id. Narrowing the dependency to one method keeps the
// test fake tiny and documents exactly what we call. The generated
// userv1.UserServiceClient satisfies it.
type userLookup interface {
	GetUser(ctx context.Context, in *userv1.GetUserRequest, opts ...grpc.CallOption) (*userv1.GetUserResponse, error)
}

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
	"order.refunded":                "order_refunded",
}

type Handler struct {
	q      *db.Queries
	users  userLookup // resolves recipient email/name (db-per-service)
	sender Sender
	log    *slog.Logger
}

func NewHandler(pool *pgxpool.Pool, users userLookup, sender Sender, log *slog.Logger) *Handler {
	return &Handler{q: db.New(pool), users: users, sender: sender, log: log}
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

	// The union of fields our event payloads carry: user_id (all), action_url
	// (verify/reset), and order fields (order.*). Fields absent for a topic stay zero.
	var data struct {
		UserID    string `json:"user_id"`
		ActionURL string `json:"action_url"`
		// verify_url is the pre-rename field name; read it as a fallback so events
		// already in the stream during a rolling deploy still carry their link.
		// TODO: remove one release after the action_url rename ships.
		VerifyURL      string `json:"verify_url"`
		OrderID        string `json:"order_id"`
		TotalCents     int64  `json:"total_cents"`
		TrackingNumber string `json:"tracking_number"`
	}
	_ = json.Unmarshal(env.Data, &data)
	if data.ActionURL == "" {
		data.ActionURL = data.VerifyURL
	}
	if data.UserID == "" {
		h.log.WarnContext(ctx, "event has no user_id; nothing to email", "event_id", env.EventID, "topic", env.Topic)
		return nil
	}

	// Claim the event in the delivery ledger FIRST — before any fallible external
	// call — so every subsequent failure (resolve or send) has a row to record on
	// and JetStream never terminates a message we haven't recorded. A brand-new
	// event inserts a pending row; a redelivery of a not-yet-sent event returns it
	// so we retry; a redelivery of an already-SENT event returns no row (ErrNoRows)
	// and we skip. This folds the dedup gate into the retry ledger.
	uid, _ := uuid.Parse(data.UserID)
	pgEvent := pgtype.UUID{Bytes: eventID, Valid: true}
	if _, err := h.q.ClaimNotificationForSend(ctx, db.ClaimNotificationForSendParams{
		EventID:  pgEvent,
		UserID:   pgtype.UUID{Bytes: uid, Valid: true},
		Channel:  "email",
		Template: template,
		Payload:  env.Data,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.log.InfoContext(ctx, "already handled (sent or dead-lettered); skipping", "event_id", env.EventID, "topic", env.Topic)
			return nil // terminal state -> idempotent skip
		}
		return err // DB down -> nak; JetStream retries (unlimited) until it recovers
	}

	// Resolve the recipient authoritatively from the User service (db-per-service:
	// the notification DB has no email). A permanent condition (unknown user, bad id)
	// is dead-lettered; a transient one retries (JetStream is unlimited).
	usr, err := h.users.GetUser(ctx, &userv1.GetUserRequest{UserId: data.UserID})
	if err != nil {
		if code := status.Code(err); code == codes.NotFound || code == codes.InvalidArgument {
			return h.deadLetter(ctx, pgEvent, env, "resolve recipient: "+err.Error())
		}
		return fmt.Errorf("resolve recipient %s: %w", data.UserID, err) // nak: retry
	}

	subject, body := Render(template, TemplateData{
		RecipientName:  usr.GetFullName(),
		ActionURL:      data.ActionURL,
		OrderID:        data.OrderID,
		TotalCents:     data.TotalCents,
		Currency:       "NGN",
		TrackingNumber: data.TrackingNumber,
	})

	// Deliver. A failed send bumps the attempt count; once it reaches the cap we
	// dead-letter (mark failed + ack) rather than retry forever.
	if err := h.sender.Send(ctx, Notification{
		EventID:  env.EventID,
		Template: template,
		To:       usr.GetEmail(),
		Subject:  subject,
		Body:     body,
	}); err != nil {
		msg := err.Error()
		attempts, rerr := h.q.RecordNotificationError(ctx, db.RecordNotificationErrorParams{EventID: pgEvent, LastError: &msg})
		if rerr != nil {
			return rerr // couldn't record the failure -> nak/retry
		}
		if attempts >= maxSendAttempts {
			return h.deadLetter(ctx, pgEvent, env, msg)
		}
		return fmt.Errorf("send notification %s: %w", env.EventID, err) // nak: retry
	}

	return h.q.MarkNotificationSent(ctx, pgEvent)
}

// deadLetter records a permanently-failed notification (status='failed') and acks
// so it stops being redelivered. If the DB write itself fails, it returns the error
// so the caller naks and retries (JetStream is unlimited) — the dead-letter is never
// lost.
func (h *Handler) deadLetter(ctx context.Context, pgEvent pgtype.UUID, env events.Envelope, reason string) error {
	if err := h.q.MarkNotificationFailed(ctx, db.MarkNotificationFailedParams{EventID: pgEvent, LastError: &reason}); err != nil {
		return err
	}
	deadLettered.Inc()
	h.log.ErrorContext(ctx, "notification dead-lettered", "event_id", env.EventID, "topic", env.Topic, "reason", reason)
	return nil // ack
}
