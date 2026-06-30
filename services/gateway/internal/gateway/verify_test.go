package gateway_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/gateway/internal/gateway"
)

// newVerifyTestServer wires an AUTHENTICATED harness with a configurable user
// client, so verification + requireVerified behaviour can be driven end to end.
func newVerifyTestServer(t *testing.T, uc *fakeUserClient, order *fakeOrderClient) (*httptest.Server, string) {
	t.Helper()
	userID := uuid.NewString()
	uc.validateFn = func(*userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
		return &userv1.ValidateTokenResponse{Valid: true, UserId: userID, Role: "customer"}, nil
	}
	h := gateway.NewHandler(uc, &fakeProductClient{}, &fakeCartClient{}, order, testMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(h.Router())
	t.Cleanup(ts.Close)
	return ts, userID
}

func TestVerifyEmail_Success(t *testing.T) {
	var gotToken string
	fake := &fakeUserClient{
		verifyEmailFn: func(in *userv1.VerifyEmailRequest) (*userv1.VerifyEmailResponse, error) {
			gotToken = in.GetToken()
			return &userv1.VerifyEmailResponse{}, nil
		},
	}
	ts := newTestServer(t, fake)

	resp := postJSON(t, ts.URL+"/auth/verify-email", `{"token":"tok-abc"}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if gotToken != "tok-abc" {
		t.Errorf("forwarded token = %q, want tok-abc", gotToken)
	}
}

func TestVerifyEmail_BadToken(t *testing.T) {
	fake := &fakeUserClient{
		verifyEmailFn: func(*userv1.VerifyEmailRequest) (*userv1.VerifyEmailResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "verification token is invalid or expired")
		},
	}
	ts := newTestServer(t, fake)

	resp := postJSON(t, ts.URL+"/auth/verify-email", `{"token":"nope"}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestVerifyEmail_MalformedBody(t *testing.T) {
	ts := newTestServer(t, &fakeUserClient{})
	resp := postJSON(t, ts.URL+"/auth/verify-email", `{`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestResendVerification_UsesIdentityUserID(t *testing.T) {
	var gotUserID string
	fake := &fakeUserClient{
		resendVerificationFn: func(in *userv1.ResendVerificationRequest) (*userv1.ResendVerificationResponse, error) {
			gotUserID = in.GetUserId()
			return &userv1.ResendVerificationResponse{}, nil
		},
	}
	ts, userID := newVerifyTestServer(t, fake, &fakeOrderClient{})

	// Even if the body tries to name another user, the gateway must use the token's id.
	req := authReq(t, http.MethodPost, ts.URL+"/auth/resend-verification", `{"user_id":"someone-else"}`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if gotUserID != userID {
		t.Errorf("forwarded user_id = %q, want the token's id %q", gotUserID, userID)
	}
}

func TestResendVerification_Unauthenticated(t *testing.T) {
	ts := newTestServer(t, &fakeUserClient{})
	resp := postJSON(t, ts.URL+"/auth/resend-verification", `{}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestRequireVerified_BlocksCheckoutForUnverified(t *testing.T) {
	placed := false
	order := &fakeOrderClient{
		placeFn: func(*orderv1.PlaceOrderRequest) (*orderv1.PlaceOrderResponse, error) {
			placed = true
			return &orderv1.PlaceOrderResponse{OrderId: "o-1", Status: "CONFIRMED"}, nil
		},
	}
	fake := &fakeUserClient{
		getUserFn: func(*userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
			return &userv1.GetUserResponse{EmailVerified: false}, nil
		},
	}
	ts, _ := newVerifyTestServer(t, fake, order)

	req := authReq(t, http.MethodPost, ts.URL+"/orders", "")
	req.Header.Set("Idempotency-Key", "key-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /orders: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
	if placed {
		t.Error("order was placed despite unverified email — checkout must be blocked before the order service")
	}
}

func TestRequireVerified_AllowsVerified(t *testing.T) {
	order := &fakeOrderClient{
		placeFn: func(*orderv1.PlaceOrderRequest) (*orderv1.PlaceOrderResponse, error) {
			return &orderv1.PlaceOrderResponse{OrderId: "o-1", Status: "CONFIRMED"}, nil
		},
	}
	fake := &fakeUserClient{
		getUserFn: func(*userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
			return &userv1.GetUserResponse{EmailVerified: true}, nil
		},
	}
	ts, _ := newVerifyTestServer(t, fake, order)

	req := authReq(t, http.MethodPost, ts.URL+"/orders", "")
	req.Header.Set("Idempotency-Key", "key-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /orders: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
}
