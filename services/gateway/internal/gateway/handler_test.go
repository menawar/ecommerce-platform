package gateway_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/menawar/ecommerce-platform/pkg/httputil"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/gateway/internal/gateway"
)

// fakeUserClient is a stub UserServiceClient. Testing the gateway against a fake
// (not the real user service) keeps the test fast and hermetic, and isolates
// exactly what the gateway is responsible for: JSON<->proto translation and
// status<->HTTP mapping. The real end-to-end is the acceptance run.
type fakeUserClient struct {
	registerFn             func(*userv1.RegisterRequest) (*userv1.RegisterResponse, error)
	loginFn                func(*userv1.LoginRequest) (*userv1.LoginResponse, error)
	validateFn             func(*userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error)
	refreshFn              func(*userv1.RefreshTokenRequest) (*userv1.RefreshTokenResponse, error)
	logoutFn               func(*userv1.LogoutRequest) (*userv1.LogoutResponse, error)
	getUserFn              func(*userv1.GetUserRequest) (*userv1.GetUserResponse, error)
	getAddressFn           func(*userv1.GetAddressRequest) (*userv1.GetAddressResponse, error)
	verifyEmailFn          func(*userv1.VerifyEmailRequest) (*userv1.VerifyEmailResponse, error)
	resendVerificationFn   func(*userv1.ResendVerificationRequest) (*userv1.ResendVerificationResponse, error)
	requestPasswordResetFn func(*userv1.RequestPasswordResetRequest) (*userv1.RequestPasswordResetResponse, error)
	resetPasswordFn        func(*userv1.ResetPasswordRequest) (*userv1.ResetPasswordResponse, error)
	createAddressFn        func(*userv1.CreateAddressRequest) (*userv1.CreateAddressResponse, error)
	listAddressesFn        func(*userv1.ListAddressesRequest) (*userv1.ListAddressesResponse, error)
	updateAddressFn        func(*userv1.UpdateAddressRequest) (*userv1.UpdateAddressResponse, error)
	deleteAddressFn        func(*userv1.DeleteAddressRequest) (*userv1.DeleteAddressResponse, error)
	setDefaultAddressFn    func(*userv1.SetDefaultAddressRequest) (*userv1.SetDefaultAddressResponse, error)
}

var _ userv1.UserServiceClient = (*fakeUserClient)(nil)

func (f *fakeUserClient) Register(_ context.Context, in *userv1.RegisterRequest, _ ...grpc.CallOption) (*userv1.RegisterResponse, error) {
	return f.registerFn(in)
}
func (f *fakeUserClient) Login(_ context.Context, in *userv1.LoginRequest, _ ...grpc.CallOption) (*userv1.LoginResponse, error) {
	return f.loginFn(in)
}
func (f *fakeUserClient) ValidateToken(_ context.Context, in *userv1.ValidateTokenRequest, _ ...grpc.CallOption) (*userv1.ValidateTokenResponse, error) {
	return f.validateFn(in)
}
func (f *fakeUserClient) GetUser(_ context.Context, in *userv1.GetUserRequest, _ ...grpc.CallOption) (*userv1.GetUserResponse, error) {
	if f.getUserFn != nil {
		return f.getUserFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeUserClient) VerifyEmail(_ context.Context, in *userv1.VerifyEmailRequest, _ ...grpc.CallOption) (*userv1.VerifyEmailResponse, error) {
	if f.verifyEmailFn != nil {
		return f.verifyEmailFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeUserClient) ResendVerification(_ context.Context, in *userv1.ResendVerificationRequest, _ ...grpc.CallOption) (*userv1.ResendVerificationResponse, error) {
	if f.resendVerificationFn != nil {
		return f.resendVerificationFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeUserClient) RequestPasswordReset(_ context.Context, in *userv1.RequestPasswordResetRequest, _ ...grpc.CallOption) (*userv1.RequestPasswordResetResponse, error) {
	if f.requestPasswordResetFn != nil {
		return f.requestPasswordResetFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeUserClient) ResetPassword(_ context.Context, in *userv1.ResetPasswordRequest, _ ...grpc.CallOption) (*userv1.ResetPasswordResponse, error) {
	if f.resetPasswordFn != nil {
		return f.resetPasswordFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeUserClient) CreateAddress(_ context.Context, in *userv1.CreateAddressRequest, _ ...grpc.CallOption) (*userv1.CreateAddressResponse, error) {
	if f.createAddressFn != nil {
		return f.createAddressFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeUserClient) ListAddresses(_ context.Context, in *userv1.ListAddressesRequest, _ ...grpc.CallOption) (*userv1.ListAddressesResponse, error) {
	if f.listAddressesFn != nil {
		return f.listAddressesFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeUserClient) GetAddress(_ context.Context, in *userv1.GetAddressRequest, _ ...grpc.CallOption) (*userv1.GetAddressResponse, error) {
	if f.getAddressFn != nil {
		return f.getAddressFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeUserClient) UpdateAddress(_ context.Context, in *userv1.UpdateAddressRequest, _ ...grpc.CallOption) (*userv1.UpdateAddressResponse, error) {
	if f.updateAddressFn != nil {
		return f.updateAddressFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeUserClient) DeleteAddress(_ context.Context, in *userv1.DeleteAddressRequest, _ ...grpc.CallOption) (*userv1.DeleteAddressResponse, error) {
	if f.deleteAddressFn != nil {
		return f.deleteAddressFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeUserClient) SetDefaultAddress(_ context.Context, in *userv1.SetDefaultAddressRequest, _ ...grpc.CallOption) (*userv1.SetDefaultAddressResponse, error) {
	if f.setDefaultAddressFn != nil {
		return f.setDefaultAddressFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeUserClient) RefreshToken(_ context.Context, in *userv1.RefreshTokenRequest, _ ...grpc.CallOption) (*userv1.RefreshTokenResponse, error) {
	if f.refreshFn != nil {
		return f.refreshFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "")
}
func (f *fakeUserClient) Logout(_ context.Context, in *userv1.LogoutRequest, _ ...grpc.CallOption) (*userv1.LogoutResponse, error) {
	if f.logoutFn != nil {
		return f.logoutFn(in)
	}
	return &userv1.LogoutResponse{}, nil
}

// testMetrics returns an HTTPMetrics instance backed by a fresh, per-test
// Prometheus registry — isolating each test's counters from the global default
// and from other tests.
func testMetrics() *httputil.HTTPMetrics {
	return httputil.NewHTTPMetrics(prometheus.NewRegistry(), "test-gateway")
}

func newTestServer(t *testing.T, fake *fakeUserClient) *httptest.Server {
	t.Helper()
	// Auth tests don't hit product/cart routes, so zero-value fakes are fine.
	h := gateway.NewHandler(fake, &fakeProductClient{}, &fakeCartClient{}, &fakeOrderClient{}, testMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(h.Router())
	t.Cleanup(ts.Close)
	return ts
}

func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// TestRouter_EchoesRequestID proves the gateway returns a correlation id the
// frontend can surface. Two cases: an inbound X-Request-Id is preserved verbatim
// (chi reuses it), and when absent the gateway still emits a non-empty one.
func TestRouter_EchoesRequestID(t *testing.T) {
	ts := newTestServer(t, &fakeUserClient{})

	t.Run("inbound id is echoed back", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/healthz", nil)
		req.Header.Set("X-Request-Id", "trace-abc-123")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /healthz: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if got := resp.Header.Get("X-Request-Id"); got != "trace-abc-123" {
			t.Errorf("X-Request-Id = %q, want it echoed as trace-abc-123", got)
		}
	})

	t.Run("generated when absent", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/healthz")
		if err != nil {
			t.Fatalf("GET /healthz: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if got := resp.Header.Get("X-Request-Id"); got == "" {
			t.Error("X-Request-Id is empty, want a generated id")
		}
	})
}

func TestRegister_Created(t *testing.T) {
	fake := &fakeUserClient{
		registerFn: func(in *userv1.RegisterRequest) (*userv1.RegisterResponse, error) {
			if in.GetEmail() != "ada@example.com" {
				t.Errorf("gateway sent wrong email: %q", in.GetEmail())
			}
			return &userv1.RegisterResponse{UserId: "u-1"}, nil
		},
	}
	ts := newTestServer(t, fake)

	resp := postJSON(t, ts.URL+"/auth/register", `{"email":"ada@example.com","password":"supersecret","full_name":"Ada"}`)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var got map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got["user_id"] != "u-1" {
		t.Errorf("user_id = %q, want u-1", got["user_id"])
	}
}

func TestRegister_DuplicateMapsTo409(t *testing.T) {
	fake := &fakeUserClient{
		registerFn: func(*userv1.RegisterRequest) (*userv1.RegisterResponse, error) {
			return nil, status.Error(codes.AlreadyExists, "email already registered")
		},
	}
	ts := newTestServer(t, fake)

	resp := postJSON(t, ts.URL+"/auth/register", `{"email":"dup@example.com","password":"supersecret","full_name":"D"}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", resp.StatusCode)
	}
}

func TestRegister_BadJSON(t *testing.T) {
	ts := newTestServer(t, &fakeUserClient{})
	resp := postJSON(t, ts.URL+"/auth/register", `{not json`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestLogin_OKReturnsTokens(t *testing.T) {
	fake := &fakeUserClient{
		loginFn: func(*userv1.LoginRequest) (*userv1.LoginResponse, error) {
			return &userv1.LoginResponse{AccessToken: "acc", RefreshToken: "ref", ExpiresAt: 123}, nil
		},
	}
	ts := newTestServer(t, fake)

	resp := postJSON(t, ts.URL+"/auth/login", `{"email":"a@b.com","password":"supersecret"}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got["access_token"] != "acc" || got["refresh_token"] != "ref" {
		t.Errorf("tokens = %+v", got)
	}
}

func TestLogin_BadCredentialsMapsTo401(t *testing.T) {
	fake := &fakeUserClient{
		loginFn: func(*userv1.LoginRequest) (*userv1.LoginResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "invalid email or password")
		},
	}
	ts := newTestServer(t, fake)

	resp := postJSON(t, ts.URL+"/auth/login", `{"email":"a@b.com","password":"nope12345"}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestRefresh_Success(t *testing.T) {
	fake := &fakeUserClient{
		refreshFn: func(in *userv1.RefreshTokenRequest) (*userv1.RefreshTokenResponse, error) {
			if in.GetRefreshToken() != "old-refresh" {
				t.Errorf("forwarded refresh token = %q", in.GetRefreshToken())
			}
			return &userv1.RefreshTokenResponse{AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresAt: 123}, nil
		},
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/auth/refresh", `{"refresh_token":"old-refresh"}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["access_token"] != "new-access" || body["refresh_token"] != "new-refresh" {
		t.Errorf("body = %v", body)
	}
}

func TestRefresh_Unauthenticated(t *testing.T) {
	fake := &fakeUserClient{
		refreshFn: func(*userv1.RefreshTokenRequest) (*userv1.RefreshTokenResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		},
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/auth/refresh", `{"refresh_token":"bad"}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestLogout_NoContent(t *testing.T) {
	called := false
	fake := &fakeUserClient{
		logoutFn: func(*userv1.LogoutRequest) (*userv1.LogoutResponse, error) {
			called = true
			return &userv1.LogoutResponse{}, nil
		},
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/auth/logout", `{"refresh_token":"x"}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if !called {
		t.Error("gateway did not forward to user.Logout")
	}
}
