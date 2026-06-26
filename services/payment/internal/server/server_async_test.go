package server_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/menawar/ecommerce-platform/pkg/postgres"
	paymentv1 "github.com/menawar/ecommerce-platform/proto/payment/v1"
	"github.com/menawar/ecommerce-platform/services/payment/internal/provider"
	"github.com/menawar/ecommerce-platform/services/payment/internal/server"
)

const webhookSecret = "test-webhook-secret"

// newAsyncServer builds a payment Server wired with the Mock async provider plus a
// real paymentdb (skips if unavailable). The async tests call the Server's methods
// directly — no gRPC plumbing needed for InitializePayment/ConfirmPayment.
func newAsyncServer(t *testing.T) (*server.Server, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("PAYMENT_DB_URL")
	if url == "" {
		url = "postgres://ecommerce:ecommerce@localhost:5433/paymentdb?sslmode=disable"
	}
	pool, err := postgres.NewPool(context.Background(), url)
	if err != nil {
		t.Skipf("skipping integration test (paymentdb unavailable; run `make infra-up && make payment-migrate-up`): %v", err)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"payments", "outbox"} {
		if _, err := pool.Exec(context.Background(), "TRUNCATE "+table); err != nil {
			t.Skipf("skipping (paymentdb not migrated for outbox; run `make payment-migrate-up`): %v", err)
		}
	}
	srv := server.NewServer(pool, slog.New(slog.NewTextHandler(io.Discard, nil))).
		WithAsync(provider.NameMock, provider.NewMock())
	return srv, pool
}

func sign(body []byte) string {
	mac := hmac.New(sha512.New, []byte(webhookSecret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// initPayment runs InitializePayment and returns the payment id + the provider
// reference stored on the row (what a real webhook would carry back).
func initPayment(t *testing.T, srv *server.Server, pool *pgxpool.Pool, amount int64) (string, string) {
	t.Helper()
	resp, err := srv.InitializePayment(context.Background(), &paymentv1.InitializePaymentRequest{
		OrderId:        uuid.NewString(),
		AmountCents:    amount,
		Currency:       "NGN",
		IdempotencyKey: uuid.NewString(),
		Email:          "buyer@example.com",
	})
	if err != nil {
		t.Fatalf("InitializePayment: %v", err)
	}
	if resp.GetStatus() != "pending" || resp.GetAuthorizationUrl() == "" {
		t.Fatalf("want pending + authorization_url, got status=%q url=%q", resp.GetStatus(), resp.GetAuthorizationUrl())
	}
	var ref string
	if err := pool.QueryRow(context.Background(),
		"SELECT provider_ref FROM payments WHERE id=$1", resp.GetPaymentId()).Scan(&ref); err != nil {
		t.Fatalf("read provider_ref: %v", err)
	}
	return resp.GetPaymentId(), ref
}

func TestInitializePayment_Idempotent(t *testing.T) {
	srv, _ := newAsyncServer(t)
	ctx := context.Background()
	req := &paymentv1.InitializePaymentRequest{
		OrderId: uuid.NewString(), AmountCents: 2500, Currency: "NGN",
		IdempotencyKey: "key-1", Email: "buyer@example.com",
	}
	first, err := srv.InitializePayment(ctx, req)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := srv.InitializePayment(ctx, req)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if first.GetPaymentId() != second.GetPaymentId() {
		t.Errorf("idempotent retry returned a different payment: %s vs %s", first.GetPaymentId(), second.GetPaymentId())
	}
}

// TestWebhook drives the full HMAC -> ConfirmPayment -> outbox path for both a
// success (amount 2500) and a deterministic decline (amount 1313).
func TestWebhook(t *testing.T) {
	cases := []struct {
		name       string
		amount     int64
		wantStatus string
		wantTopic  string
	}{
		{"success", 2500, "succeeded", "payment.succeeded"},
		{"decline", 1313, "failed", "payment.failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, pool := newAsyncServer(t)
			ts := httptest.NewServer(srv.PaystackWebhookHandler(webhookSecret))
			defer ts.Close()

			paymentID, ref := initPayment(t, srv, pool, tc.amount)

			body, _ := json.Marshal(map[string]any{
				"event": "charge.success",
				"data":  map[string]any{"reference": ref},
			})
			req, _ := http.NewRequest(http.MethodPost, ts.URL, bytes.NewReader(body))
			req.Header.Set("x-paystack-signature", sign(body))
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("post webhook: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("webhook status = %d, want 200", resp.StatusCode)
			}

			var gotStatus string
			if err := pool.QueryRow(context.Background(),
				"SELECT status FROM payments WHERE id=$1", paymentID).Scan(&gotStatus); err != nil {
				t.Fatalf("read status: %v", err)
			}
			if gotStatus != tc.wantStatus {
				t.Errorf("payment status = %q, want %q", gotStatus, tc.wantStatus)
			}

			var gotTopic string
			if err := pool.QueryRow(context.Background(),
				"SELECT topic FROM outbox ORDER BY created_at DESC LIMIT 1").Scan(&gotTopic); err != nil {
				t.Fatalf("read outbox: %v", err)
			}
			if gotTopic != tc.wantTopic {
				t.Errorf("outbox topic = %q, want %q", gotTopic, tc.wantTopic)
			}
		})
	}
}

func TestWebhook_BadSignatureRejected(t *testing.T) {
	srv, pool := newAsyncServer(t)
	ts := httptest.NewServer(srv.PaystackWebhookHandler(webhookSecret))
	defer ts.Close()

	_, ref := initPayment(t, srv, pool, 2500)
	body, _ := json.Marshal(map[string]any{"event": "charge.success", "data": map[string]any{"reference": ref}})

	req, _ := http.NewRequest(http.MethodPost, ts.URL, bytes.NewReader(body))
	req.Header.Set("x-paystack-signature", "deadbeef") // wrong
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post webhook: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}

	// The payment must remain pending — a forged webhook changes nothing.
	var status string
	_ = pool.QueryRow(context.Background(), "SELECT status FROM payments WHERE provider_ref=$1", ref).Scan(&status)
	if status != "pending" {
		t.Errorf("status = %q after forged webhook, want pending", status)
	}
}
