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

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cartv1 "github.com/menawar/ecommerce-platform/proto/cart/v1"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/gateway/internal/gateway"
)

// fakeCartClient stubs CartServiceClient; tests set only the methods they exercise.
type fakeCartClient struct {
	getFn    func(*cartv1.GetCartRequest) (*cartv1.GetCartResponse, error)
	addFn    func(*cartv1.AddItemRequest) (*cartv1.AddItemResponse, error)
	updateFn func(*cartv1.UpdateItemRequest) (*cartv1.UpdateItemResponse, error)
	removeFn func(*cartv1.RemoveItemRequest) (*cartv1.RemoveItemResponse, error)
}

var _ cartv1.CartServiceClient = (*fakeCartClient)(nil)

func (f *fakeCartClient) GetCart(_ context.Context, in *cartv1.GetCartRequest, _ ...grpc.CallOption) (*cartv1.GetCartResponse, error) {
	return f.getFn(in)
}
func (f *fakeCartClient) AddItem(_ context.Context, in *cartv1.AddItemRequest, _ ...grpc.CallOption) (*cartv1.AddItemResponse, error) {
	return f.addFn(in)
}
func (f *fakeCartClient) UpdateItem(_ context.Context, in *cartv1.UpdateItemRequest, _ ...grpc.CallOption) (*cartv1.UpdateItemResponse, error) {
	return f.updateFn(in)
}
func (f *fakeCartClient) RemoveItem(_ context.Context, in *cartv1.RemoveItemRequest, _ ...grpc.CallOption) (*cartv1.RemoveItemResponse, error) {
	return f.removeFn(in)
}
func (f *fakeCartClient) ClearCart(context.Context, *cartv1.ClearCartRequest, ...grpc.CallOption) (*cartv1.ClearCartResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unused")
}

// newCartTestServer wires an AUTHENTICATED harness: the user fake's ValidateToken
// returns valid + a fixed userID, so requireAuth populates the Identity. Returns
// the userID so tests can assert the gateway forwarded the TOKEN's id (not a
// client-supplied one).
func newCartTestServer(t *testing.T, cart *fakeCartClient) (*httptest.Server, string) {
	t.Helper()
	userID := uuid.NewString()
	uc := &fakeUserClient{
		validateFn: func(*userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
			return &userv1.ValidateTokenResponse{Valid: true, UserId: userID, Role: "customer"}, nil
		},
	}
	h := gateway.NewHandler(uc, &fakeProductClient{}, cart, &fakeOrderClient{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(h.Router())
	t.Cleanup(ts.Close)
	return ts, userID
}

func authReq(t *testing.T, method, url, body string) *http.Request {
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
	r.Header.Set("Authorization", "Bearer test-token") // fake validateFn accepts any
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestGetCart_UsesTokenUserID(t *testing.T) {
	var gotUserID string
	cart := &fakeCartClient{
		getFn: func(in *cartv1.GetCartRequest) (*cartv1.GetCartResponse, error) {
			gotUserID = in.GetUserId()
			return &cartv1.GetCartResponse{Cart: &cartv1.Cart{UserId: in.GetUserId(), Items: []*cartv1.CartItem{
				{ProductId: "p1", Quantity: 3},
			}}}, nil
		},
	}
	ts, userID := newCartTestServer(t, cart)

	resp, err := http.DefaultClient.Do(authReq(t, http.MethodGet, ts.URL+"/cart", ""))
	if err != nil {
		t.Fatalf("GET /cart: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// The forwarded user_id is the TOKEN's id, proving the client can't spoof it.
	if gotUserID != userID {
		t.Errorf("forwarded user_id = %q, want token id %q", gotUserID, userID)
	}

	var body cartBody
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Items) != 1 || body.Items[0].ProductID != "p1" || body.Items[0].Quantity != 3 {
		t.Errorf("cart body = %+v", body)
	}
}

func TestAddCartItem_ForwardsBody(t *testing.T) {
	var got *cartv1.AddItemRequest
	cart := &fakeCartClient{
		addFn: func(in *cartv1.AddItemRequest) (*cartv1.AddItemResponse, error) {
			got = in
			return &cartv1.AddItemResponse{Cart: &cartv1.Cart{Items: []*cartv1.CartItem{{ProductId: in.GetProductId(), Quantity: in.GetQuantity()}}}}, nil
		},
	}
	ts, userID := newCartTestServer(t, cart)

	resp, err := http.DefaultClient.Do(authReq(t, http.MethodPost, ts.URL+"/cart/items", `{"product_id":"p9","quantity":4}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got.GetUserId() != userID || got.GetProductId() != "p9" || got.GetQuantity() != 4 {
		t.Errorf("forwarded = %+v", got)
	}
}

func TestUpdateAndRemove_UseURLParam(t *testing.T) {
	var updatedPID, removedPID string
	cart := &fakeCartClient{
		updateFn: func(in *cartv1.UpdateItemRequest) (*cartv1.UpdateItemResponse, error) {
			updatedPID = in.GetProductId()
			return &cartv1.UpdateItemResponse{Cart: &cartv1.Cart{}}, nil
		},
		removeFn: func(in *cartv1.RemoveItemRequest) (*cartv1.RemoveItemResponse, error) {
			removedPID = in.GetProductId()
			return &cartv1.RemoveItemResponse{Cart: &cartv1.Cart{}}, nil
		},
	}
	ts, _ := newCartTestServer(t, cart)

	up, _ := http.DefaultClient.Do(authReq(t, http.MethodPut, ts.URL+"/cart/items/pX", `{"quantity":7}`))
	up.Body.Close()
	if updatedPID != "pX" {
		t.Errorf("update product = %q, want pX", updatedPID)
	}

	rm, _ := http.DefaultClient.Do(authReq(t, http.MethodDelete, ts.URL+"/cart/items/pY", ""))
	rm.Body.Close()
	if removedPID != "pY" {
		t.Errorf("remove product = %q, want pY", removedPID)
	}
}

func TestCart_RequiresAuth(t *testing.T) {
	ts, _ := newCartTestServer(t, &fakeCartClient{})
	// No Authorization header -> requireAuth rejects with 401.
	resp, err := http.Get(ts.URL + "/cart")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

type cartBody struct {
	Items []struct {
		ProductID string `json:"product_id"`
		Quantity  int32  `json:"quantity"`
	} `json:"items"`
}
