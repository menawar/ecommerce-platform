package server

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PaystackWebhookHandler authenticates a Paystack webhook and finalizes the
// referenced payment. Paystack signs every webhook as HMAC-SHA512 of the raw
// request body, keyed by the account's secret key, in the x-paystack-signature
// header — so a caller who doesn't hold the secret can't forge one.
//
// The signature is defense-in-depth, not the sole gate: ConfirmPayment ALWAYS
// re-verifies the transaction with the provider before acting, so even a valid-
// looking body can't confirm a payment that Paystack didn't actually charge.
func (s *Server) PaystackWebhookHandler(secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read the raw body BEFORE decoding: the HMAC is over the exact bytes, so
		// re-serializing parsed JSON would change them and break verification.
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}

		mac := hmac.New(sha512.New, []byte(secret))
		mac.Write(body)
		want := hex.EncodeToString(mac.Sum(nil))
		got := r.Header.Get("x-paystack-signature")
		if !hmac.Equal([]byte(got), []byte(want)) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		var evt struct {
			Event string `json:"event"`
			Data  struct {
				Reference string `json:"reference"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &evt); err != nil || evt.Data.Reference == "" {
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}

		if err := s.ConfirmPayment(r.Context(), evt.Data.Reference); err != nil {
			if status.Code(err) == codes.NotFound {
				// Unknown reference: ack with 200 so Paystack stops retrying — there
				// is nothing for us to do with a payment we never started.
				w.WriteHeader(http.StatusOK)
				return
			}
			// Transient/internal failure: 500 so Paystack retries the webhook later.
			s.log.ErrorContext(r.Context(), "webhook confirm failed", "err", err, "reference", evt.Data.Reference)
			http.Error(w, "internal", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
