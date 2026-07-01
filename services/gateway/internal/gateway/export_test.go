package gateway_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/gateway/internal/gateway"
)

func newExportServer(t *testing.T, uc *fakeUserClient, oc *fakeOrderClient) (*httptest.Server, string) {
	t.Helper()
	userID := uuid.NewString()
	uc.validateFn = func(*userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
		return &userv1.ValidateTokenResponse{Valid: true, UserId: userID, Role: "customer"}, nil
	}
	h := gateway.NewHandler(uc, &fakeProductClient{}, &fakeCartClient{}, oc, testMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(h.Router())
	t.Cleanup(ts.Close)
	return ts, userID
}

func TestExportData_AggregatesAndDownloads(t *testing.T) {
	uc := &fakeUserClient{
		getUserFn: func(in *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
			return &userv1.GetUserResponse{UserId: in.GetUserId(), Email: "ada@x.com", FullName: "Ada", Role: "customer", EmailVerified: true}, nil
		},
		listAddressesFn: func(*userv1.ListAddressesRequest) (*userv1.ListAddressesResponse, error) {
			return &userv1.ListAddressesResponse{Addresses: []*userv1.Address{{Id: "a-1", Recipient: "Ada"}}}, nil
		},
	}
	oc := &fakeOrderClient{
		listFn: func(*orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
			return &orderv1.ListOrdersResponse{Orders: []*orderv1.Order{{Id: "o-1", Status: "CONFIRMED"}}}, nil
		},
		// Export fetches full order detail (with items) per id via GetOrder.
		getFn: func(in *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
			return &orderv1.GetOrderResponse{Order: &orderv1.Order{
				Id:     in.GetOrderId(),
				Status: "CONFIRMED",
				Items:  []*orderv1.OrderItem{{ProductId: "p-1", Quantity: 2}},
			}}, nil
		},
	}
	ts, userID := newExportServer(t, uc, oc)

	req := authReq(t, http.MethodGet, ts.URL+"/me/export", "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /me/export: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd == "" {
		t.Error("missing Content-Disposition (should download as a file)")
	}

	var out struct {
		ExportedAt string `json:"exported_at"`
		Profile    struct {
			UserID string `json:"user_id"`
			Email  string `json:"email"`
		} `json:"profile"`
		Addresses []map[string]any `json:"addresses"`
		Orders    []struct {
			ID    string           `json:"id"`
			Items []map[string]any `json:"items"`
		} `json:"orders"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ExportedAt == "" {
		t.Error("exported_at should be set")
	}
	if out.Profile.UserID != userID || out.Profile.Email != "ada@x.com" {
		t.Errorf("profile = %+v, want user %s / ada@x.com", out.Profile, userID)
	}
	if len(out.Addresses) != 1 || len(out.Orders) != 1 {
		t.Fatalf("want 1 address + 1 order; got %d / %d", len(out.Addresses), len(out.Orders))
	}
	// The export must include line items (fetched via GetOrder), not just summaries.
	if len(out.Orders[0].Items) != 1 {
		t.Errorf("order should include its line items; got %d", len(out.Orders[0].Items))
	}
}

// A full page must trigger a follow-up page — the export has to be complete.
func TestExportData_PagesThroughAllOrders(t *testing.T) {
	uc := &fakeUserClient{
		getUserFn: func(in *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
			return &userv1.GetUserResponse{UserId: in.GetUserId()}, nil
		},
		listAddressesFn: func(*userv1.ListAddressesRequest) (*userv1.ListAddressesResponse, error) {
			return &userv1.ListAddressesResponse{}, nil
		},
	}
	var pages []int32
	oc := &fakeOrderClient{
		listFn: func(in *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
			pages = append(pages, in.GetPage())
			if in.GetPage() == 1 {
				full := make([]*orderv1.Order, in.GetPageSize()) // a full page => keep paging
				for i := range full {
					full[i] = &orderv1.Order{Id: fmt.Sprintf("o-%d", i)}
				}
				return &orderv1.ListOrdersResponse{Orders: full}, nil
			}
			return &orderv1.ListOrdersResponse{Orders: []*orderv1.Order{{Id: "o-last"}}}, nil // partial => stop
		},
		getFn: func(in *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
			return &orderv1.GetOrderResponse{Order: &orderv1.Order{Id: in.GetOrderId()}}, nil
		},
	}
	ts, _ := newExportServer(t, uc, oc)

	req := authReq(t, http.MethodGet, ts.URL+"/me/export", "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /me/export: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var out struct {
		Orders []map[string]any `json:"orders"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if len(pages) != 2 || pages[0] != 1 || pages[1] != 2 {
		t.Errorf("expected to page 1 then 2, got %v", pages)
	}
	if len(out.Orders) < 2 {
		t.Errorf("expected all pages aggregated, got %d orders", len(out.Orders))
	}
}

func TestDeleteAccount_ForwardsIdentity(t *testing.T) {
	var got *userv1.DeleteUserRequest
	uc := &fakeUserClient{
		deleteUserFn: func(in *userv1.DeleteUserRequest) (*userv1.DeleteUserResponse, error) {
			got = in
			return &userv1.DeleteUserResponse{}, nil
		},
	}
	ts, userID := newExportServer(t, uc, &fakeOrderClient{})

	req := authReq(t, http.MethodPost, ts.URL+"/me/delete", "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /me/delete: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	// The deleted user_id must come from the JWT, never the request body.
	if got.GetUserId() != userID {
		t.Errorf("deleted user_id = %q, want %s (from token)", got.GetUserId(), userID)
	}
}

func TestDeleteAccount_RequiresAuth(t *testing.T) {
	ts, _ := newExportServer(t, &fakeUserClient{}, &fakeOrderClient{})
	resp, err := http.Post(ts.URL+"/me/delete", "application/json", nil) // no token
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestExportData_RequiresAuth(t *testing.T) {
	ts, _ := newExportServer(t, &fakeUserClient{}, &fakeOrderClient{})
	resp, err := http.Get(ts.URL + "/me/export") // no token
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}
