package notify_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/menawar/ecommerce-platform/pkg/events"
	"github.com/menawar/ecommerce-platform/pkg/postgres"
	"github.com/menawar/ecommerce-platform/services/notification/internal/db"
	"github.com/menawar/ecommerce-platform/services/notification/internal/notify"
)

type countingSender struct{ calls int32 }

func (s *countingSender) Send(context.Context, notify.Notification) error {
	atomic.AddInt32(&s.calls, 1)
	return nil
}

// capturingSender records the last Notification it was asked to send.
type capturingSender struct{ last notify.Notification }

func (s *capturingSender) Send(_ context.Context, n notify.Notification) error {
	s.last = n
	return nil
}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("NOTIFICATION_DB_URL")
	if url == "" {
		url = "postgres://ecommerce:ecommerce@localhost:5433/notificationdb?sslmode=disable"
	}
	pool, err := postgres.NewPool(context.Background(), url)
	if err != nil {
		t.Skipf("skipping integration test (notificationdb unavailable): %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), "TRUNCATE notifications"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func event(t *testing.T, topic string) events.Envelope {
	t.Helper()
	env, err := events.New(topic, map[string]string{"user_id": uuid.NewString()})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	return env
}

func countFor(t *testing.T, pool *pgxpool.Pool, eventID string) int64 {
	t.Helper()
	id, _ := uuid.Parse(eventID)
	n, err := db.New(pool).CountByEventID(context.Background(), pgtype.UUID{Bytes: id, Valid: true})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// TestHandle_Idempotent is the core Phase 5 guarantee: the SAME event delivered
// twice produces exactly ONE notification and ONE send.
func TestHandle_Idempotent(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	sender := &countingSender{}
	h := notify.NewHandler(pool, sender, discard())
	env := event(t, "order.confirmed")

	if err := h.Handle(ctx, env); err != nil {
		t.Fatalf("first Handle: %v", err)
	}
	if err := h.Handle(ctx, env); err != nil { // redelivery
		t.Fatalf("replay Handle: %v", err)
	}

	if got := countFor(t, pool, env.EventID); got != 1 {
		t.Errorf("notifications for event = %d, want 1 (duplicate not deduped)", got)
	}
	if n := atomic.LoadInt32(&sender.calls); n != 1 {
		t.Errorf("sender called %d times, want 1", n)
	}
}

// TestHandle_ActionLinkSurfaced proves the action link (action_url) rides from the
// event payload through to the Sender for transactional-link emails, so the email
// can include it (and the LogSender prints it in dev).
func TestHandle_ActionLinkSurfaced(t *testing.T) {
	cases := []struct {
		topic, link, wantTemplate string
	}{
		{"user.verification_requested", "http://localhost:3000/verify-email?token=abc-123", "email_verification"},
		{"user.password_reset_requested", "http://localhost:3000/reset-password?token=xyz-789", "password_reset"},
	}
	for _, c := range cases {
		t.Run(c.topic, func(t *testing.T) {
			ctx := context.Background()
			pool := testPool(t)
			sender := &capturingSender{}
			h := notify.NewHandler(pool, sender, discard())

			env, err := events.New(c.topic, map[string]string{
				"user_id":    uuid.NewString(),
				"action_url": c.link,
			})
			if err != nil {
				t.Fatalf("event: %v", err)
			}
			if err := h.Handle(ctx, env); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if sender.last.Template != c.wantTemplate {
				t.Errorf("template = %q, want %q", sender.last.Template, c.wantTemplate)
			}
			if sender.last.Link != c.link {
				t.Errorf("link = %q, want %q", sender.last.Link, c.link)
			}
		})
	}
}

func TestHandle_IgnoresUnknownTopic(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	sender := &countingSender{}
	h := notify.NewHandler(pool, sender, discard())
	env := event(t, "order.archived") // not in topicTemplates

	if err := h.Handle(ctx, env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got := countFor(t, pool, env.EventID); got != 0 {
		t.Errorf("unknown topic created %d notifications, want 0", got)
	}
	if atomic.LoadInt32(&sender.calls) != 0 {
		t.Error("sender should not be called for an unhandled topic")
	}
}

func TestHandle_TemplatesByTopic(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	h := notify.NewHandler(pool, &countingSender{}, discard())

	cases := map[string]string{
		"user.registered":             "welcome",
		"user.verification_requested": "email_verification",
		"order.confirmed":             "order_confirmation",
		"order.cancelled":             "order_cancelled",
		"order.paid":                  "payment_received",
		"order.shipped":               "order_shipped",
		"order.delivered":             "order_delivered",
	}
	for topic, wantTemplate := range cases {
		env := event(t, topic)
		if err := h.Handle(ctx, env); err != nil {
			t.Fatalf("Handle %s: %v", topic, err)
		}
		id, _ := uuid.Parse(env.EventID)
		rows, _ := db.New(pool).ListByUser(ctx, db.ListByUserParams{Limit: 100})
		_ = rows // smoke; template assertion below via direct query
		var template string
		_ = pool.QueryRow(ctx, "SELECT template FROM notifications WHERE event_id=$1", pgtype.UUID{Bytes: id, Valid: true}).Scan(&template)
		if template != wantTemplate {
			t.Errorf("topic %s -> template %q, want %q", topic, template, wantTemplate)
		}
	}
}
