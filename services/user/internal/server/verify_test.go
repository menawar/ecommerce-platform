package server_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/menawar/ecommerce-platform/pkg/auth"
	"github.com/menawar/ecommerce-platform/pkg/events"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/user/internal/server"
	"github.com/menawar/ecommerce-platform/services/user/internal/store"
)

const testWebBaseURL = "http://localhost:3000"

// capturePublisher records the most recent payload per topic so a test can read
// back the verification link the server emitted.
type capturePublisher struct {
	mu      sync.Mutex
	byTopic map[string][]byte
}

func newCapturePublisher() *capturePublisher {
	return &capturePublisher{byTopic: make(map[string][]byte)}
}

func (c *capturePublisher) Publish(_ context.Context, topic string, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.byTopic[topic] = payload
	return nil
}

// tokenFor parses the captured event for the given topic and pulls the token out
// of its action_url. It fails the test if no such event was emitted.
func (c *capturePublisher) tokenFor(t *testing.T, topic string) string {
	t.Helper()
	c.mu.Lock()
	payload, ok := c.byTopic[topic]
	c.mu.Unlock()
	if !ok {
		t.Fatalf("no %s event was published", topic)
	}
	env, err := events.Parse(payload)
	if err != nil {
		t.Fatalf("parse envelope: %v", err)
	}
	var data struct {
		ActionURL string `json:"action_url"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	u, err := url.Parse(data.ActionURL)
	if err != nil {
		t.Fatalf("parse action_url %q: %v", data.ActionURL, err)
	}
	tok := u.Query().Get("token")
	if tok == "" {
		t.Fatalf("action_url %q has no token", data.ActionURL)
	}
	return tok
}

// published reports whether an event for the topic was captured (for asserting a
// NON-event, e.g. enumeration-safe password reset for an unknown email).
func (c *capturePublisher) published(topic string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.byTopic[topic]
	return ok
}

// newVerifyTestClient wires a Server with a real in-memory store and a capturing
// publisher, returning the client and the publisher so tests can recover tokens.
func newVerifyTestClient(t *testing.T) (userv1.UserServiceClient, *capturePublisher) {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	jwtMgr := auth.NewJWTManager("test-secret", 15*time.Minute, auth.TypeAccess)
	refreshMgr := auth.NewJWTManager("test-secret", 7*24*time.Hour, auth.TypeRefresh)
	pub := newCapturePublisher()
	srv := server.NewServer(
		store.NewMemory(), store.NewMemoryRefreshTokens(), store.NewMemoryVerificationTokens(),
		store.NewMemoryPasswordResetTokens(),
		jwtMgr, refreshMgr, jwtMgr, refreshMgr, pub, testWebBaseURL, log,
	)

	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	userv1.RegisterUserServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return userv1.NewUserServiceClient(conn), pub
}

func registerForVerify(t *testing.T, client userv1.UserServiceClient, email string) string {
	t.Helper()
	reg, err := client.Register(context.Background(), &userv1.RegisterRequest{
		Email: email, Password: "supersecret", FullName: "Ada Lovelace",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	return reg.GetUserId()
}

func TestVerifyEmail_HappyPath(t *testing.T) {
	ctx := context.Background()
	client, pub := newVerifyTestClient(t)

	userID := registerForVerify(t, client, "ada@example.com")

	// New account starts unverified.
	got, err := client.GetUser(ctx, &userv1.GetUserRequest{UserId: userID})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.GetEmailVerified() {
		t.Fatal("new account should be unverified")
	}

	token := pub.tokenFor(t, "user.verification_requested")
	if _, err := client.VerifyEmail(ctx, &userv1.VerifyEmailRequest{Token: token}); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}

	got, _ = client.GetUser(ctx, &userv1.GetUserRequest{UserId: userID})
	if !got.GetEmailVerified() {
		t.Error("account should be verified after VerifyEmail")
	}
}

func TestVerifyEmail_ReclickIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client, pub := newVerifyTestClient(t)
	registerForVerify(t, client, "ada@example.com")
	token := pub.tokenFor(t, "user.verification_requested")

	if _, err := client.VerifyEmail(ctx, &userv1.VerifyEmailRequest{Token: token}); err != nil {
		t.Fatalf("first VerifyEmail: %v", err)
	}
	// Same link clicked again: the token is now spent but the account is verified,
	// so this is a success, not an error.
	if _, err := client.VerifyEmail(ctx, &userv1.VerifyEmailRequest{Token: token}); err != nil {
		t.Errorf("re-click VerifyEmail should succeed, got %v", err)
	}
}

func TestVerifyEmail_BadToken(t *testing.T) {
	ctx := context.Background()
	client, _ := newVerifyTestClient(t)

	cases := []struct {
		name, token string
	}{
		{"empty", ""},
		{"unknown", "00000000-0000-0000-0000-000000000000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := client.VerifyEmail(ctx, &userv1.VerifyEmailRequest{Token: c.token})
			if status.Code(err) != codes.InvalidArgument {
				t.Errorf("want InvalidArgument, got %v", err)
			}
		})
	}
}

func TestResendVerification(t *testing.T) {
	ctx := context.Background()
	client, pub := newVerifyTestClient(t)
	userID := registerForVerify(t, client, "ada@example.com")

	// Resend issues a fresh, usable token that verifies the account.
	if _, err := client.ResendVerification(ctx, &userv1.ResendVerificationRequest{UserId: userID}); err != nil {
		t.Fatalf("ResendVerification: %v", err)
	}
	token := pub.tokenFor(t, "user.verification_requested")
	if _, err := client.VerifyEmail(ctx, &userv1.VerifyEmailRequest{Token: token}); err != nil {
		t.Fatalf("VerifyEmail after resend: %v", err)
	}

	// Resending to an already-verified account is a no-op success.
	if _, err := client.ResendVerification(ctx, &userv1.ResendVerificationRequest{UserId: userID}); err != nil {
		t.Errorf("ResendVerification on verified account should succeed, got %v", err)
	}
}

func TestResendVerification_BadInput(t *testing.T) {
	ctx := context.Background()
	client, _ := newVerifyTestClient(t)

	if _, err := client.ResendVerification(ctx, &userv1.ResendVerificationRequest{UserId: "not-a-uuid"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("want InvalidArgument, got %v", err)
	}
	if _, err := client.ResendVerification(ctx, &userv1.ResendVerificationRequest{UserId: "00000000-0000-0000-0000-000000000000"}); status.Code(err) != codes.NotFound {
		t.Errorf("want NotFound, got %v", err)
	}
}
