package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/menawar/ecommerce-platform/services/payment/internal/provider"
)

// fakePaystack stands in for Paystack's API: it asserts auth + payload on
// initialize and lets each test script the verify status. No network, no keys.
func fakePaystack(t *testing.T, verifyStatus string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/transaction/initialize", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk_test_x" {
			t.Errorf("Authorization = %q, want bearer secret key", got)
		}
		var body struct {
			Email     string `json:"email"`
			Amount    int64  `json:"amount"`
			Reference string `json:"reference"`
			Currency  string `json:"currency"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Email == "" || body.Amount == 0 || body.Reference == "" {
			t.Errorf("initialize got incomplete body: %+v", body)
		}
		writeJSON(w, map[string]any{
			"status":  true,
			"message": "Authorization URL created",
			"data": map[string]any{
				"authorization_url": "https://checkout.paystack.com/" + body.Reference,
				"access_code":       "ac_123",
				"reference":         body.Reference,
			},
		})
	})

	mux.HandleFunc("/transaction/verify/", func(w http.ResponseWriter, r *http.Request) {
		ref := strings.TrimPrefix(r.URL.Path, "/transaction/verify/")
		if ref == "" {
			t.Error("verify called without a reference")
		}
		writeJSON(w, map[string]any{
			"status":  true,
			"message": "Verification successful",
			"data":    map[string]any{"status": verifyStatus, "reference": ref},
		})
	})

	return httptest.NewServer(mux)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestPaystackInitialize(t *testing.T) {
	srv := fakePaystack(t, "success")
	defer srv.Close()

	p := provider.NewPaystack("sk_test_x", provider.WithPaystackBaseURL(srv.URL))
	url, ref, err := p.Initialize(context.Background(), 50000, "NGN", "order-9", "buyer@example.com")
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if ref != "order-9" {
		t.Errorf("providerRef = %q, want echoed reference order-9", ref)
	}
	if !strings.HasPrefix(url, "https://checkout.paystack.com/") {
		t.Errorf("authorization url = %q, want paystack checkout url", url)
	}
}

// TestPaystackVerify maps Paystack's transaction status onto our three values.
func TestPaystackVerify(t *testing.T) {
	cases := map[string]string{
		"success":   provider.StatusSucceeded,
		"failed":    provider.StatusFailed,
		"abandoned": provider.StatusFailed,
		"ongoing":   provider.StatusPending,
	}
	for psStatus, want := range cases {
		t.Run(psStatus, func(t *testing.T) {
			srv := fakePaystack(t, psStatus)
			defer srv.Close()

			p := provider.NewPaystack("sk_test_x", provider.WithPaystackBaseURL(srv.URL))
			got, err := p.Verify(context.Background(), "order-9")
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if got != want {
				t.Errorf("Verify(paystack=%q) = %q, want %q", psStatus, got, want)
			}
		})
	}
}

// TestPaystackAPIError: a status=false envelope (e.g. invalid key) must surface as
// an error, never a usable result.
func TestPaystackAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"status": false, "message": "Invalid key"})
	}))
	defer srv.Close()

	p := provider.NewPaystack("sk_test_x", provider.WithPaystackBaseURL(srv.URL))
	if _, _, err := p.Initialize(context.Background(), 1000, "NGN", "o1", "e@x.com"); err == nil {
		t.Fatal("Initialize: want error on status=false envelope, got nil")
	}
}
