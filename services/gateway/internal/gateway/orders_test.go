package gateway_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/gateway/internal/gateway"
)

type fakeOrderClient struct {
	placeFn          func(*orderv1.PlaceOrderRequest) (*orderv1.PlaceOrderResponse, error)
	listFn           func(*orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error)
	getFn            func(*orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error)
	listShippingFn   func(*orderv1.ListShippingMethodsRequest) (*orderv1.ListShippingMethodsResponse, error)
	createShippingFn func(*orderv1.CreateShippingMethodRequest) (*orderv1.CreateShippingMethodResponse, error)
	updateShippingFn func(*orderv1.UpdateShippingMethodRequest) (*orderv1.UpdateShippingMethodResponse, error)
	deleteShippingFn func(*orderv1.DeleteShippingMethodRequest) (*orderv1.DeleteShippingMethodResponse, error)
}

var _ orderv1.OrderServiceClient = (*fakeOrderClient)(nil)

func (f *fakeOrderClient) PlaceOrder(_ context.Context, in *orderv1.PlaceOrderRequest, _ ...grpc.CallOption) (*orderv1.PlaceOrderResponse, error) {
	return f.placeFn(in)
}
func (f *fakeOrderClient) ListOrders(_ context.Context, in *orderv1.ListOrdersRequest, _ ...grpc.CallOption) (*orderv1.ListOrdersResponse, error) {
	return f.listFn(in)
}
func (f *fakeOrderClient) GetOrder(_ context.Context, in *orderv1.GetOrderRequest, _ ...grpc.CallOption) (*orderv1.GetOrderResponse, error) {
	return f.getFn(in)
}
func (f *fakeOrderClient) CancelOrder(context.Context, *orderv1.CancelOrderRequest, ...grpc.CallOption) (*orderv1.CancelOrderResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unused")
}
func (f *fakeOrderClient) ListShippingMethods(_ context.Context, in *orderv1.ListShippingMethodsRequest, _ ...grpc.CallOption) (*orderv1.ListShippingMethodsResponse, error) {
	if f.listShippingFn != nil {
		return f.listShippingFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "unused")
}
func (f *fakeOrderClient) CreateShippingMethod(_ context.Context, in *orderv1.CreateShippingMethodRequest, _ ...grpc.CallOption) (*orderv1.CreateShippingMethodResponse, error) {
	if f.createShippingFn != nil {
		return f.createShippingFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "unused")
}
func (f *fakeOrderClient) UpdateShippingMethod(_ context.Context, in *orderv1.UpdateShippingMethodRequest, _ ...grpc.CallOption) (*orderv1.UpdateShippingMethodResponse, error) {
	if f.updateShippingFn != nil {
		return f.updateShippingFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "unused")
}
func (f *fakeOrderClient) DeleteShippingMethod(_ context.Context, in *orderv1.DeleteShippingMethodRequest, _ ...grpc.CallOption) (*orderv1.DeleteShippingMethodResponse, error) {
	if f.deleteShippingFn != nil {
		return f.deleteShippingFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "unused")
}

func newOrderTestServer(t *testing.T, order *fakeOrderClient) (*httptest.Server, string) {
	t.Helper()
	userID := uuid.NewString()
	uc := &fakeUserClient{
		validateFn: func(*userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
			return &userv1.ValidateTokenResponse{Valid: true, UserId: userID, Role: "customer"}, nil
		},
		// placeOrder is gated behind requireVerified, which calls GetUser — the
		// order tests assume a verified customer so checkout isn't blocked.
		getUserFn: func(*userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
			return &userv1.GetUserResponse{UserId: userID, EmailVerified: true}, nil
		},
	}
	h := gateway.NewHandler(uc, &fakeProductClient{}, &fakeCartClient{}, order, testMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(h.Router())
	t.Cleanup(ts.Close)
	return ts, userID
}

func TestPlaceOrder_ForwardsTokenUserAndIdempotencyKey(t *testing.T) {
	var got *orderv1.PlaceOrderRequest
	order := &fakeOrderClient{
		placeFn: func(in *orderv1.PlaceOrderRequest) (*orderv1.PlaceOrderResponse, error) {
			got = in
			return &orderv1.PlaceOrderResponse{OrderId: "o-1", Status: "CONFIRMED"}, nil
		},
	}
	ts, userID := newOrderTestServer(t, order)

	req := authReq(t, http.MethodPost, ts.URL+"/orders", "")
	req.Header.Set("Idempotency-Key", "key-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /orders: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if got.GetUserId() != userID || got.GetIdempotencyKey() != "key-123" {
		t.Errorf("forwarded = %+v (want user %s, key key-123)", got, userID)
	}
}

func TestPlaceOrder_MissingIdempotencyKey(t *testing.T) {
	ts, _ := newOrderTestServer(t, &fakeOrderClient{})
	resp, err := http.DefaultClient.Do(authReq(t, http.MethodPost, ts.URL+"/orders", ""))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing Idempotency-Key: status = %d, want 400", resp.StatusCode)
	}
}

// TestGetOrder_OwnershipCheck: a user cannot read another user's order — the
// gateway returns 404 when the order's user_id != the caller's.
func TestGetOrder_OwnershipCheck(t *testing.T) {
	order := &fakeOrderClient{
		getFn: func(in *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
			// Belongs to SOMEONE ELSE.
			return &orderv1.GetOrderResponse{Order: &orderv1.Order{Id: in.GetOrderId(), UserId: uuid.NewString(), Status: "CONFIRMED"}}, nil
		},
	}
	ts, _ := newOrderTestServer(t, order)

	resp, err := http.DefaultClient.Do(authReq(t, http.MethodGet, ts.URL+"/orders/o-1", ""))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("other user's order: status = %d, want 404", resp.StatusCode)
	}
}

func TestListOrders_OK(t *testing.T) {
	order := &fakeOrderClient{
		listFn: func(in *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
			return &orderv1.ListOrdersResponse{Orders: []*orderv1.Order{
				{Id: "o-1", UserId: in.GetUserId(), Status: "CONFIRMED", TotalCents: 5000},
			}}, nil
		},
	}
	ts, _ := newOrderTestServer(t, order)

	resp, err := http.DefaultClient.Do(authReq(t, http.MethodGet, ts.URL+"/orders", ""))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Orders []map[string]any `json:"orders"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Orders) != 1 || body.Orders[0]["status"] != "CONFIRMED" {
		t.Errorf("orders = %+v", body.Orders)
	}
}
