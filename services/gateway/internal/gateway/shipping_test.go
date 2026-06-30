package gateway_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/gateway/internal/gateway"
)

// newShippingTestServer maps "admin-token" → admin role, anything else → customer,
// so one server serves both the role-aware list and the admin-gated mutations.
func newShippingTestServer(t *testing.T, oc *fakeOrderClient) *httptest.Server {
	t.Helper()
	uc := &fakeUserClient{
		validateFn: func(in *userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
			role := "customer"
			if in.GetToken() == "admin-token" {
				role = "admin"
			}
			return &userv1.ValidateTokenResponse{Valid: true, UserId: "u1", Role: role}, nil
		},
	}
	h := gateway.NewHandler(uc, &fakeProductClient{}, &fakeCartClient{}, oc, testMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(h.Router())
	t.Cleanup(ts.Close)
	return ts
}

func doReq(t *testing.T, method, url, token, body string) *http.Response {
	t.Helper()
	var r *http.Request
	var err error
	if body == "" {
		r, err = http.NewRequest(method, url, nil)
	} else {
		r, err = http.NewRequest(method, url, strings.NewReader(body))
	}
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func TestListShippingMethods_RoleAware(t *testing.T) {
	var gotActiveOnly bool
	oc := &fakeOrderClient{
		listShippingFn: func(in *orderv1.ListShippingMethodsRequest) (*orderv1.ListShippingMethodsResponse, error) {
			gotActiveOnly = in.GetActiveOnly()
			return &orderv1.ListShippingMethodsResponse{Methods: []*orderv1.ShippingMethod{{Id: "s1", Name: "Standard"}}}, nil
		},
	}
	ts := newShippingTestServer(t, oc)

	t.Run("customer gets active_only=true (checkout view)", func(t *testing.T) {
		resp := doReq(t, http.MethodGet, ts.URL+"/shipping-methods", "cust-token", "")
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if !gotActiveOnly {
			t.Error("customer should request active_only=true")
		}
		var out struct {
			ShippingMethods []map[string]any `json:"shipping_methods"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		if len(out.ShippingMethods) != 1 {
			t.Errorf("methods = %+v, want 1", out.ShippingMethods)
		}
	})

	t.Run("admin without ?all still gets active_only (their own checkout)", func(t *testing.T) {
		resp := doReq(t, http.MethodGet, ts.URL+"/shipping-methods", "admin-token", "")
		defer func() { _ = resp.Body.Close() }()
		if !gotActiveOnly {
			t.Error("admin without ?all should still get active_only=true")
		}
	})

	t.Run("admin with ?all=true gets the full list", func(t *testing.T) {
		resp := doReq(t, http.MethodGet, ts.URL+"/shipping-methods?all=true", "admin-token", "")
		defer func() { _ = resp.Body.Close() }()
		if gotActiveOnly {
			t.Error("admin with ?all=true should get active_only=false")
		}
	})

	t.Run("customer with ?all=true is still active_only (can't see disabled)", func(t *testing.T) {
		resp := doReq(t, http.MethodGet, ts.URL+"/shipping-methods?all=true", "cust-token", "")
		defer func() { _ = resp.Body.Close() }()
		if !gotActiveOnly {
			t.Error("customer must not be able to coerce the full list via ?all=true")
		}
	})
}

func TestCreateShippingMethod_AdminGate(t *testing.T) {
	var got *orderv1.CreateShippingMethodRequest
	oc := &fakeOrderClient{
		createShippingFn: func(in *orderv1.CreateShippingMethodRequest) (*orderv1.CreateShippingMethodResponse, error) {
			got = in
			return &orderv1.CreateShippingMethodResponse{Method: &orderv1.ShippingMethod{Id: "s1", Name: in.GetMethod().GetName()}}, nil
		},
	}
	ts := newShippingTestServer(t, oc)
	const body = `{"name":"Express","description":"2 days","price_cents":350000,"sort_order":2,"active":true}`

	t.Run("admin creates (201, forwarded)", func(t *testing.T) {
		resp := doReq(t, http.MethodPost, ts.URL+"/shipping-methods", "admin-token", body)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want 201", resp.StatusCode)
		}
		if got.GetMethod().GetName() != "Express" || got.GetMethod().GetPriceCents() != 350000 {
			t.Errorf("forwarded = %+v", got.GetMethod())
		}
	})

	t.Run("customer forbidden (403)", func(t *testing.T) {
		resp := doReq(t, http.MethodPost, ts.URL+"/shipping-methods", "cust-token", body)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want 403", resp.StatusCode)
		}
	})

	t.Run("unauthenticated (401)", func(t *testing.T) {
		resp := doReq(t, http.MethodPost, ts.URL+"/shipping-methods", "", body)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})
}

func TestUpdateShippingMethod_ForwardsID(t *testing.T) {
	var got *orderv1.UpdateShippingMethodRequest
	oc := &fakeOrderClient{
		updateShippingFn: func(in *orderv1.UpdateShippingMethodRequest) (*orderv1.UpdateShippingMethodResponse, error) {
			got = in
			return &orderv1.UpdateShippingMethodResponse{Method: &orderv1.ShippingMethod{Id: in.GetId()}}, nil
		},
	}
	ts := newShippingTestServer(t, oc)

	resp := doReq(t, http.MethodPatch, ts.URL+"/shipping-methods/s-9", "admin-token", `{"name":"Std","price_cents":1500,"active":true}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got.GetId() != "s-9" || got.GetMethod().GetName() != "Std" {
		t.Errorf("forwarded = id %s method %+v", got.GetId(), got.GetMethod())
	}
}

func TestDeleteShippingMethod_UnknownMapsTo404(t *testing.T) {
	oc := &fakeOrderClient{
		deleteShippingFn: func(*orderv1.DeleteShippingMethodRequest) (*orderv1.DeleteShippingMethodResponse, error) {
			return nil, status.Error(codes.NotFound, "shipping method not found")
		},
	}
	ts := newShippingTestServer(t, oc)

	resp := doReq(t, http.MethodDelete, ts.URL+"/shipping-methods/s-x", "admin-token", "")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
