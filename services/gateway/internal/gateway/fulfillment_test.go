package gateway_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
)

// Reuses newShippingTestServer (shipping_test.go): "admin-token" → admin role.

func TestMarkShipped_AdminGateAndForwarding(t *testing.T) {
	var got *orderv1.MarkShippedRequest
	oc := &fakeOrderClient{
		markShippedFn: func(in *orderv1.MarkShippedRequest) (*orderv1.MarkShippedResponse, error) {
			got = in
			return &orderv1.MarkShippedResponse{Order: &orderv1.Order{Id: in.GetOrderId(), Status: "SHIPPED", TrackingNumber: in.GetTrackingNumber()}}, nil
		},
	}
	ts := newShippingTestServer(t, oc)

	t.Run("admin ships (200, forwards id+tracking)", func(t *testing.T) {
		resp := doReq(t, http.MethodPost, ts.URL+"/orders/o-1/ship", "admin-token", `{"tracking_number":"TRACK-9"}`)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if got.GetOrderId() != "o-1" || got.GetTrackingNumber() != "TRACK-9" {
			t.Errorf("forwarded = %+v", got)
		}
		var dto map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&dto)
		if dto["status"] != "SHIPPED" {
			t.Errorf("status = %v, want SHIPPED", dto["status"])
		}
	})

	t.Run("customer forbidden (403)", func(t *testing.T) {
		resp := doReq(t, http.MethodPost, ts.URL+"/orders/o-1/ship", "cust-token", `{}`)
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want 403", resp.StatusCode)
		}
	})
}

func TestMarkDelivered_Forwards(t *testing.T) {
	var got *orderv1.MarkDeliveredRequest
	oc := &fakeOrderClient{
		markDeliveredFn: func(in *orderv1.MarkDeliveredRequest) (*orderv1.MarkDeliveredResponse, error) {
			got = in
			return &orderv1.MarkDeliveredResponse{Order: &orderv1.Order{Id: in.GetOrderId(), Status: "DELIVERED"}}, nil
		},
	}
	ts := newShippingTestServer(t, oc)

	resp := doReq(t, http.MethodPost, ts.URL+"/orders/o-7/deliver", "admin-token", "")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got.GetOrderId() != "o-7" {
		t.Errorf("forwarded id = %q, want o-7", got.GetOrderId())
	}
}

func TestMarkShipped_IllegalTransitionMapsTo422(t *testing.T) {
	oc := &fakeOrderClient{
		markShippedFn: func(*orderv1.MarkShippedRequest) (*orderv1.MarkShippedResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "order in PENDING cannot move to SHIPPED")
		},
	}
	ts := newShippingTestServer(t, oc)

	resp := doReq(t, http.MethodPost, ts.URL+"/orders/o-1/ship", "admin-token", `{}`)
	defer func() { _ = resp.Body.Close() }()
	// writeGRPCError maps FailedPrecondition → 422 Unprocessable Entity.
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestRefundOrder_AdminGateAndForwarding(t *testing.T) {
	var got *orderv1.RefundOrderRequest
	oc := &fakeOrderClient{
		refundOrderFn: func(in *orderv1.RefundOrderRequest) (*orderv1.RefundOrderResponse, error) {
			got = in
			return &orderv1.RefundOrderResponse{Order: &orderv1.Order{Id: in.GetOrderId(), Status: "REFUNDED"}}, nil
		},
	}
	ts := newShippingTestServer(t, oc)

	t.Run("admin refunds (200, forwards id)", func(t *testing.T) {
		resp := doReq(t, http.MethodPost, ts.URL+"/orders/o-1/refund", "admin-token", "")
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if got.GetOrderId() != "o-1" {
			t.Errorf("forwarded id = %q, want o-1", got.GetOrderId())
		}
	})

	t.Run("customer forbidden (403)", func(t *testing.T) {
		resp := doReq(t, http.MethodPost, ts.URL+"/orders/o-1/refund", "cust-token", "")
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want 403", resp.StatusCode)
		}
	})
}

func TestRefundOrder_NonRefundableMapsTo422(t *testing.T) {
	oc := &fakeOrderClient{
		refundOrderFn: func(*orderv1.RefundOrderRequest) (*orderv1.RefundOrderResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "order in PAYMENT_PENDING cannot be refunded")
		},
	}
	ts := newShippingTestServer(t, oc)

	resp := doReq(t, http.MethodPost, ts.URL+"/orders/o-1/refund", "admin-token", "")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestListAllOrders_AdminOnly(t *testing.T) {
	oc := &fakeOrderClient{
		listAllOrdersFn: func(*orderv1.ListAllOrdersRequest) (*orderv1.ListAllOrdersResponse, error) {
			return &orderv1.ListAllOrdersResponse{Orders: []*orderv1.Order{{Id: "o-1", Status: "CONFIRMED"}}}, nil
		},
	}
	ts := newShippingTestServer(t, oc)

	t.Run("admin sees all orders", func(t *testing.T) {
		resp := doReq(t, http.MethodGet, ts.URL+"/admin/orders", "admin-token", "")
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var out struct {
			Orders []map[string]any `json:"orders"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		if len(out.Orders) != 1 {
			t.Errorf("orders = %+v, want 1", out.Orders)
		}
	})

	t.Run("customer forbidden", func(t *testing.T) {
		resp := doReq(t, http.MethodGet, ts.URL+"/admin/orders", "cust-token", "")
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want 403", resp.StatusCode)
		}
	})
}
