package gateway_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/gateway/internal/gateway"
)

// newAddrTestServer wires an authenticated harness with a fixed user, returning
// the server and the user id the token resolves to.
func newAddrTestServer(t *testing.T, uc *fakeUserClient) (*httptest.Server, string) {
	t.Helper()
	userID := uuid.NewString()
	uc.validateFn = func(*userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
		return &userv1.ValidateTokenResponse{Valid: true, UserId: userID, Role: "customer"}, nil
	}
	h := gateway.NewHandler(uc, &fakeProductClient{}, &fakeCartClient{}, &fakeOrderClient{}, testMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(h.Router())
	t.Cleanup(ts.Close)
	return ts, userID
}

func TestCreateAddress_ForwardsIdentityAndBody(t *testing.T) {
	var got *userv1.CreateAddressRequest
	uc := &fakeUserClient{
		createAddressFn: func(in *userv1.CreateAddressRequest) (*userv1.CreateAddressResponse, error) {
			got = in
			return &userv1.CreateAddressResponse{Address: &userv1.Address{Id: "a-1", Recipient: in.GetAddress().GetRecipient(), IsDefault: in.GetIsDefault()}}, nil
		},
	}
	ts, userID := newAddrTestServer(t, uc)

	req := authReq(t, http.MethodPost, ts.URL+"/addresses", `{"recipient":"Ada","phone":"0803","line1":"1 Rayfield","city":"Jos","state":"Plateau","is_default":true}`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /addresses: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	// user_id comes from the token, recipient + is_default from the body.
	if got.GetUserId() != userID || got.GetAddress().GetRecipient() != "Ada" || !got.GetIsDefault() {
		t.Errorf("forwarded = %+v (want user %s, recipient Ada, default true)", got, userID)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out["id"] != "a-1" {
		t.Errorf("response id = %v, want a-1", out["id"])
	}
}

func TestListAddresses_ReturnsArray(t *testing.T) {
	uc := &fakeUserClient{
		listAddressesFn: func(*userv1.ListAddressesRequest) (*userv1.ListAddressesResponse, error) {
			return &userv1.ListAddressesResponse{Addresses: []*userv1.Address{
				{Id: "a-1", Recipient: "Ada", IsDefault: true},
			}}, nil
		},
	}
	ts, _ := newAddrTestServer(t, uc)

	resp, err := http.DefaultClient.Do(authReq(t, http.MethodGet, ts.URL+"/addresses", ""))
	if err != nil {
		t.Fatalf("GET /addresses: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		Addresses []map[string]any `json:"addresses"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Addresses) != 1 || out.Addresses[0]["id"] != "a-1" {
		t.Errorf("addresses = %+v, want one with id a-1", out.Addresses)
	}
}

func TestUpdateAddress_ForwardsIDAndUser(t *testing.T) {
	var got *userv1.UpdateAddressRequest
	uc := &fakeUserClient{
		updateAddressFn: func(in *userv1.UpdateAddressRequest) (*userv1.UpdateAddressResponse, error) {
			got = in
			return &userv1.UpdateAddressResponse{}, nil
		},
	}
	ts, userID := newAddrTestServer(t, uc)

	req := authReq(t, http.MethodPatch, ts.URL+"/addresses/a-9", `{"recipient":"Ada","phone":"0803","line1":"1","city":"Jos","state":"Plateau"}`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if got.GetUserId() != userID || got.GetAddressId() != "a-9" {
		t.Errorf("forwarded = user %s id %s, want %s / a-9", got.GetUserId(), got.GetAddressId(), userID)
	}
}

func TestDeleteAddress_UnknownMapsTo404(t *testing.T) {
	uc := &fakeUserClient{
		deleteAddressFn: func(*userv1.DeleteAddressRequest) (*userv1.DeleteAddressResponse, error) {
			return nil, status.Error(codes.NotFound, "address not found")
		},
	}
	ts, _ := newAddrTestServer(t, uc)

	resp, err := http.DefaultClient.Do(authReq(t, http.MethodDelete, ts.URL+"/addresses/a-x", ""))
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSetDefaultAddress_Forwards(t *testing.T) {
	var got *userv1.SetDefaultAddressRequest
	uc := &fakeUserClient{
		setDefaultAddressFn: func(in *userv1.SetDefaultAddressRequest) (*userv1.SetDefaultAddressResponse, error) {
			got = in
			return &userv1.SetDefaultAddressResponse{}, nil
		},
	}
	ts, userID := newAddrTestServer(t, uc)

	resp, err := http.DefaultClient.Do(authReq(t, http.MethodPost, ts.URL+"/addresses/a-2/default", ""))
	if err != nil {
		t.Fatalf("POST default: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if got.GetUserId() != userID || got.GetAddressId() != "a-2" {
		t.Errorf("forwarded = user %s id %s, want %s / a-2", got.GetUserId(), got.GetAddressId(), userID)
	}
}

func TestAddresses_RequireAuth(t *testing.T) {
	ts, _ := newAddrTestServer(t, &fakeUserClient{})
	// No Authorization header → requireAuth rejects before any handler runs.
	resp, err := http.Get(ts.URL + "/addresses")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}
