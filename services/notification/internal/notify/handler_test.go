package notify_test

import (
	"context"
	"errors"
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

// flakySender fails its first failN sends, then succeeds — to exercise retry.
type flakySender struct {
	failN, calls int
}

func (s *flakySender) Send(context.Context, notify.Notification) error {
	s.calls++
	if s.calls <= s.failN {
		return errors.New("smtp down")
	}
	return nil
}

// failSender always fails — to exercise dead-lettering.
type failSender struct{ calls int }

func (s *failSender) Send(context.Context, notify.Notification) error {
	s.calls++
	return errors.New("smtp down")
}

func statusFor(t *testing.T, pool *pgxpool.Pool, eventID string) string {
	t.Helper()
	id, _ := uuid.Parse(eventID)
	var s string
	_ = pool.QueryRow(context.Background(), "SELECT status FROM notifications WHERE event_id=$1",
		pgtype.UUID{Bytes: id, Valid: true}).Scan(&s)
	return s
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

// TestHandle_ResolveFailureRetries: a transient GetUser failure naks (retry) and
// leaves the ledger row pending (not sent, not dead-lettered) so the redelivery
// re-attempts it — the email is never lost.
func TestHandle_ResolveFailureRetries(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	sender := &countingSender{}
	h := notify.NewHandler(pool, errUser{err: status.Error(codes.Unavailable, "user svc down")}, sender, discard())
	env := event(t, "order.confirmed")

	if err := h.Handle(ctx, env); err == nil {
		t.Fatal("transient resolve failure should return an error (nak/retry)")
	}
	if got := statusFor(t, pool, env.EventID); got != "pending" {
		t.Errorf("after transient resolve failure status = %s, want pending (will retry)", got)
	}
	if atomic.LoadInt32(&sender.calls) != 0 {
		t.Error("sender must not be called when resolve fails")
	}
}

// TestHandle_UnknownRecipientDropped: a NotFound recipient is permanent — the row
// is dead-lettered (status=failed) and acked (no error, no infinite retry), no send.
func TestHandle_UnknownRecipientDropped(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	sender := &countingSender{}
	h := notify.NewHandler(pool, errUser{err: status.Error(codes.NotFound, "no such user")}, sender, discard())
	env := event(t, "order.confirmed")

	if err := h.Handle(ctx, env); err != nil {
		t.Fatalf("NotFound recipient should ack (nil), got %v", err)
	}
	if got := statusFor(t, pool, env.EventID); got != "failed" {
		t.Errorf("unknown recipient status = %s, want failed (dead-lettered)", got)
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

// TestHandle_RetriesFailedSend is the Phase 13.3 headline: a send that fails is
// NOT lost. The first delivery fails (naks, row stays pending); the redelivery
// re-attempts the same row and succeeds — one row, exactly-once eventual delivery.
func TestHandle_RetriesFailedSend(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	sender := &flakySender{failN: 1}
	h := notify.NewHandler(pool, fakeUser{}, sender, discard())
	env := event(t, "order.confirmed")

	if err := h.Handle(ctx, env); err == nil {
		t.Fatal("first send fails -> Handle should return an error (nak)")
	}
	if got := statusFor(t, pool, env.EventID); got != "pending" {
		t.Errorf("after failed send status = %s, want pending", got)
	}
	if err := h.Handle(ctx, env); err != nil { // redelivery
		t.Fatalf("retry should succeed: %v", err)
	}
	if got := statusFor(t, pool, env.EventID); got != "sent" {
		t.Errorf("after retry status = %s, want sent", got)
	}
	if sender.calls != 2 {
		t.Errorf("sends = %d, want 2 (fail then succeed)", sender.calls)
	}
	if countFor(t, pool, env.EventID) != 1 {
		t.Error("retry must reuse the one ledger row, not create a second")
	}
}

// TestHandle_DeadLettersAfterMax: a permanently-failing send is retried up to the
// cap, then dead-lettered (status=failed, ack) instead of retried forever.
func TestHandle_DeadLettersAfterMax(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	sender := &failSender{}
	h := notify.NewHandler(pool, fakeUser{}, sender, discard())
	env := event(t, "order.confirmed")

	const maxAttempts = 5 // matches notify.maxSendAttempts / JetStream MaxDeliver
	for i := 1; i <= maxAttempts; i++ {
		err := h.Handle(ctx, env)
		if i < maxAttempts && err == nil {
			t.Fatalf("attempt %d should nak (return error)", i)
		}
		if i == maxAttempts && err != nil {
			t.Fatalf("final attempt should ack (dead-letter), got %v", err)
		}
	}
	if got := statusFor(t, pool, env.EventID); got != "failed" {
		t.Errorf("status = %s, want failed (dead-lettered)", got)
	}
	if sender.calls != maxAttempts {
		t.Errorf("sends = %d, want %d", sender.calls, maxAttempts)
	}
}

// TestHandle_DeadLetterIsTerminal: once dead-lettered, a redelivery is skipped —
// not reprocessed/re-sent — even if the sender would now succeed.
func TestHandle_DeadLetterIsTerminal(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	failing := &failSender{}
	h := notify.NewHandler(pool, fakeUser{}, failing, discard())
	env := event(t, "order.confirmed")

	const maxAttempts = 5
	for range maxAttempts {
		_ = h.Handle(ctx, env)
	}
	if got := statusFor(t, pool, env.EventID); got != "failed" {
		t.Fatalf("precondition: status = %s, want failed", got)
	}

	// Redeliver to a now-healthy sender: must be skipped (terminal), not re-sent.
	healthy := &countingSender{}
	h2 := notify.NewHandler(pool, fakeUser{}, healthy, discard())
	if err := h2.Handle(ctx, env); err != nil {
		t.Fatalf("redelivery of a dead-lettered event should ack, got %v", err)
	}
	if atomic.LoadInt32(&healthy.calls) != 0 {
		t.Error("a dead-lettered event must not be re-sent on redelivery")
	}
	if got := statusFor(t, pool, env.EventID); got != "failed" {
		t.Errorf("status = %s, want failed (still terminal)", got)
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
