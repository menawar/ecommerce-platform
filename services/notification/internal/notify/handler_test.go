package notify_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/menawar/ecommerce-platform/pkg/events"
	"github.com/menawar/ecommerce-platform/pkg/postgres"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/notification/internal/db"
	"github.com/menawar/ecommerce-platform/services/notification/internal/notify"
)

// fakeUser is a minimal recipient resolver (satisfies the handler's userLookup):
// it returns a fixed email + name for any id.
type fakeUser struct{}

func (fakeUser) GetUser(_ context.Context, _ *userv1.GetUserRequest, _ ...grpc.CallOption) (*userv1.GetUserResponse, error) {
	return &userv1.GetUserResponse{Email: "buyer@example.com", FullName: "Ada Lovelace"}, nil
}

// errUser fails GetUser with a fixed error (to exercise the resolve-failure paths).
type errUser struct{ err error }

func (u errUser) GetUser(_ context.Context, _ *userv1.GetUserRequest, _ ...grpc.CallOption) (*userv1.GetUserResponse, error) {
	return nil, u.err
}

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
	h := notify.NewHandler(pool, fakeUser{}, sender, discard())
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

// TestHandle_RendersEmailWithLink proves the recipient is resolved (via the User
// service) and the action link is rendered into the email body for transactional
// emails — the whole point of Phase 13.1.
func TestHandle_RendersEmailWithLink(t *testing.T) {
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
			h := notify.NewHandler(pool, fakeUser{}, sender, discard())

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
			if sender.last.To != "buyer@example.com" {
				t.Errorf("to = %q, want the resolved recipient", sender.last.To)
			}
			if sender.last.Subject == "" {
				t.Error("subject should be rendered")
			}
			if !strings.Contains(sender.last.Body, c.link) {
				t.Errorf("body should contain the action link %q; got %q", c.link, sender.last.Body)
			}
		})
	}
}

// TestHandle_ResolveFailureRetries: a transient GetUser failure must NOT record a
// dedup row (or the redelivery would be swallowed and the email lost) — it returns
// an error so JetStream retries, and nothing is inserted.
func TestHandle_ResolveFailureRetries(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	sender := &countingSender{}
	h := notify.NewHandler(pool, errUser{err: status.Error(codes.Unavailable, "user svc down")}, sender, discard())
	env := event(t, "order.confirmed")

	if err := h.Handle(ctx, env); err == nil {
		t.Fatal("transient resolve failure should return an error (nak/retry)")
	}
	if got := countFor(t, pool, env.EventID); got != 0 {
		t.Errorf("no dedup row should be written on resolve failure; got %d", got)
	}
	if atomic.LoadInt32(&sender.calls) != 0 {
		t.Error("sender must not be called when resolve fails")
	}
}

// TestHandle_UnknownRecipientDropped: a NotFound recipient is permanent — ack (no
// error, no infinite retry), no send.
func TestHandle_UnknownRecipientDropped(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	sender := &countingSender{}
	h := notify.NewHandler(pool, errUser{err: status.Error(codes.NotFound, "no such user")}, sender, discard())
	env := event(t, "order.confirmed")

	if err := h.Handle(ctx, env); err != nil {
		t.Fatalf("NotFound recipient should ack (nil), got %v", err)
	}
	if atomic.LoadInt32(&sender.calls) != 0 {
		t.Error("sender must not be called for an unknown recipient")
	}
}

// TestHandle_ShippedIncludesTracking: the order.shipped event carries the tracking
// number, and it lands in the rendered email body.
func TestHandle_ShippedIncludesTracking(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	sender := &capturingSender{}
	h := notify.NewHandler(pool, fakeUser{}, sender, discard())
	env, err := events.New("order.shipped", map[string]any{
		"user_id": uuid.NewString(), "order_id": "o-1", "tracking_number": "TRK-9",
	})
	if err != nil {
		t.Fatalf("event: %v", err)
	}
	if err := h.Handle(ctx, env); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !strings.Contains(sender.last.Body, "TRK-9") {
		t.Errorf("shipped email should include tracking; got %q", sender.last.Body)
	}
}

func TestHandle_IgnoresUnknownTopic(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	sender := &countingSender{}
	h := notify.NewHandler(pool, fakeUser{}, sender, discard())
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
	h := notify.NewHandler(pool, fakeUser{}, &countingSender{}, discard())

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
