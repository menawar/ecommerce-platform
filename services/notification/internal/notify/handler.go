package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/menawar/ecommerce-platform/pkg/events"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/notification/internal/db"
)

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

	// Resolve the recipient authoritatively from the User service (db-per-service:
	// the notification DB has no email). Do this BEFORE the dedup insert so a
	// failure re-runs cleanly on redelivery instead of being swallowed by the dedup
	// row. NotFound is permanent (deleted/unknown user) -> ack; other errors are
	// transient -> nak/retry.
	usr, err := h.users.GetUser(ctx, &userv1.GetUserRequest{UserId: data.UserID})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			h.log.WarnContext(ctx, "recipient not found; dropping notification", "event_id", env.EventID, "user_id", data.UserID)
			return nil
		}
		return fmt.Errorf("resolve recipient %s: %w", data.UserID, err)
	}

	subject, body := Render(template, TemplateData{
		RecipientName:  usr.GetFullName(),
		ActionURL:      data.ActionURL,
		OrderID:        data.OrderID,
		TotalCents:     data.TotalCents,
		Currency:       "NGN",
		TrackingNumber: data.TrackingNumber,
	})

	// The dedup gate: insert keyed by the UNIQUE event_id. A duplicate delivery
	// fails here, and we treat that as "already handled" — the heart of an
	// idempotent consumer.
	uid, _ := uuid.Parse(data.UserID)
	err = h.q.InsertNotification(ctx, db.InsertNotificationParams{
		EventID:  pgtype.UUID{Bytes: eventID, Valid: true},
		UserID:   pgtype.UUID{Bytes: uid, Valid: true},
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

	return h.sender.Send(ctx, Notification{
		EventID:  env.EventID,
		Template: template,
		To:       usr.GetEmail(),
		Subject:  subject,
		Body:     body,
	})
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
