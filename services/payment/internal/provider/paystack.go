package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultPaystackBaseURL is Paystack's live API host. Tests override it to point at
// an httptest server; production reads it from config.
const defaultPaystackBaseURL = "https://api.paystack.co"

// Paystack is an AsyncProvider backed by Paystack's REST API (the PSP we use for
// NGN — Stripe doesn't onboard Nigerian businesses). It speaks two endpoints:
// POST /transaction/initialize (start) and GET /transaction/verify/{ref} (the
// authoritative result we trust over any webhook payload).
//
// It implements only AsyncProvider, never the legacy sync Provider — a real PSP
// cannot return the outcome inline.
type Paystack struct {
	secretKey  string
	baseURL    string
	httpClient *http.Client
}

var _ AsyncProvider = (*Paystack)(nil)

// PaystackOption tweaks a Paystack for tests (base URL) or callers (HTTP client).
type PaystackOption func(*Paystack)

// WithPaystackBaseURL overrides the API host — used by tests to target httptest.
func WithPaystackBaseURL(u string) PaystackOption {
	return func(p *Paystack) { p.baseURL = u }
}

// WithPaystackHTTPClient injects a custom client (timeouts, transport, tracing).
func WithPaystackHTTPClient(c *http.Client) PaystackOption {
	return func(p *Paystack) { p.httpClient = c }
}

// NewPaystack builds a Paystack provider. secretKey is the Paystack secret key
// (sk_test_… in test mode, sk_live_… in production).
func NewPaystack(secretKey string, opts ...PaystackOption) *Paystack {
	p := &Paystack{
		secretKey:  secretKey,
		baseURL:    defaultPaystackBaseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// paystackEnvelope is Paystack's uniform response wrapper. status=false means the
// API rejected the call; data carries the endpoint-specific payload.
type paystackEnvelope struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// Initialize creates a transaction. Paystack wants the amount in the currency's
// minor unit (kobo for NGN) — which is exactly how we carry amountCents — and a
// reference it will echo back on the webhook and from Verify.
func (p *Paystack) Initialize(ctx context.Context, amountCents int64, currency, ref, email string) (string, string, error) {
	body, err := json.Marshal(map[string]any{
		"email":     email,
		"amount":    amountCents,
		"currency":  currency,
		"reference": ref,
	})
	if err != nil {
		return "", "", fmt.Errorf("paystack initialize: marshal request: %w", err)
	}

	env, err := p.do(ctx, http.MethodPost, "/transaction/initialize", body)
	if err != nil {
		return "", "", err
	}

	var data struct {
		AuthorizationURL string `json:"authorization_url"`
		Reference        string `json:"reference"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return "", "", fmt.Errorf("paystack initialize: decode data: %w", err)
	}
	if data.AuthorizationURL == "" {
		return "", "", fmt.Errorf("paystack initialize: empty authorization_url")
	}
	// Prefer Paystack's reference; fall back to the one we sent (they match in
	// practice, but trust the provider's id as the source of truth).
	providerRef := data.Reference
	if providerRef == "" {
		providerRef = ref
	}
	return data.AuthorizationURL, providerRef, nil
}

// Verify pulls the authoritative status for a reference. We map Paystack's
// transaction status onto our three values; only "success" is a confirmed charge.
func (p *Paystack) Verify(ctx context.Context, providerRef string) (string, error) {
	env, err := p.do(ctx, http.MethodGet, "/transaction/verify/"+providerRef, nil)
	if err != nil {
		return "", err
	}
	var data struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return "", fmt.Errorf("paystack verify: decode data: %w", err)
	}
	switch data.Status {
	case "success":
		return StatusSucceeded, nil
	case "failed", "abandoned", "reversed":
		return StatusFailed, nil
	default:
		// ongoing / pending — the customer hasn't finished authorizing yet.
		return StatusPending, nil
	}
}

// do performs an authenticated Paystack request and unwraps the standard
// envelope, turning a non-2xx response or status=false into an error so callers
// only deal with the happy data payload.
func (p *Paystack) do(ctx context.Context, method, path string, body []byte) (*paystackEnvelope, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("paystack %s %s: build request: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+p.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paystack %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("paystack %s %s: read body: %w", method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("paystack %s %s: http %d: %s", method, path, resp.StatusCode, raw)
	}

	var env paystackEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("paystack %s %s: decode envelope: %w", method, path, err)
	}
	if !env.Status {
		return nil, fmt.Errorf("paystack %s %s: api error: %s", method, path, env.Message)
	}
	return &env, nil
}
