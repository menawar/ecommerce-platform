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

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
	"github.com/menawar/ecommerce-platform/services/gateway/internal/gateway"
)

// fakeProductClient stubs ProductServiceClient. Only List/Get are exercised by the
// gateway's public routes; the saga RPCs are present to satisfy the interface.
type fakeProductClient struct {
	listFn   func(*productv1.ListProductsRequest) (*productv1.ListProductsResponse, error)
	getFn    func(*productv1.GetProductRequest) (*productv1.GetProductResponse, error)
	createFn func(*productv1.CreateProductRequest) (*productv1.CreateProductResponse, error)
	updateFn func(*productv1.UpdateProductRequest) (*productv1.UpdateProductResponse, error)
	deleteFn func(*productv1.DeleteProductRequest) (*productv1.DeleteProductResponse, error)
}

var _ productv1.ProductServiceClient = (*fakeProductClient)(nil)

func (f *fakeProductClient) ListProducts(_ context.Context, in *productv1.ListProductsRequest, _ ...grpc.CallOption) (*productv1.ListProductsResponse, error) {
	return f.listFn(in)
}
func (f *fakeProductClient) GetProduct(_ context.Context, in *productv1.GetProductRequest, _ ...grpc.CallOption) (*productv1.GetProductResponse, error) {
	return f.getFn(in)
}
func (f *fakeProductClient) CreateProduct(_ context.Context, in *productv1.CreateProductRequest, _ ...grpc.CallOption) (*productv1.CreateProductResponse, error) {
	if f.createFn != nil {
		return f.createFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "unused")
}
func (f *fakeProductClient) UpdateProduct(_ context.Context, in *productv1.UpdateProductRequest, _ ...grpc.CallOption) (*productv1.UpdateProductResponse, error) {
	if f.updateFn != nil {
		return f.updateFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "unused")
}
func (f *fakeProductClient) DeleteProduct(_ context.Context, in *productv1.DeleteProductRequest, _ ...grpc.CallOption) (*productv1.DeleteProductResponse, error) {
	if f.deleteFn != nil {
		return f.deleteFn(in)
	}
	return nil, status.Error(codes.Unimplemented, "unused")
}
func (f *fakeProductClient) ReserveStock(context.Context, *productv1.ReserveStockRequest, ...grpc.CallOption) (*productv1.ReserveStockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unused")
}
func (f *fakeProductClient) ReleaseStock(context.Context, *productv1.ReleaseStockRequest, ...grpc.CallOption) (*productv1.ReleaseStockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unused")
}
func (f *fakeProductClient) CommitStock(context.Context, *productv1.CommitStockRequest, ...grpc.CallOption) (*productv1.CommitStockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unused")
}

func newProductTestServer(t *testing.T, fake *fakeProductClient) *httptest.Server {
	t.Helper()
	h := gateway.NewHandler(&fakeUserClient{}, fake, &fakeCartClient{}, &fakeOrderClient{}, testMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(h.Router())
	t.Cleanup(ts.Close)
	return ts
}

func TestListProducts_ForwardsParamsAndShapesJSON(t *testing.T) {
	var gotReq *productv1.ListProductsRequest
	fake := &fakeProductClient{
		listFn: func(in *productv1.ListProductsRequest) (*productv1.ListProductsResponse, error) {
			gotReq = in
			return &productv1.ListProductsResponse{
				Products: []*productv1.Product{{Id: "p1", Sku: "SKU1", Name: "Widget", PriceCents: 1999, Available: 5}},
				Total:    1,
			}, nil
		},
	}
	ts := newProductTestServer(t, fake)

	resp, err := http.Get(ts.URL + "/products?page=2&page_size=5&q=wid&category_id=cat1&sort=price_asc")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Params were forwarded to the gRPC request.
	if gotReq.GetPage() != 2 || gotReq.GetPageSize() != 5 || gotReq.GetSearch() != "wid" || gotReq.GetCategoryId() != "cat1" || gotReq.GetSort() != "price_asc" {
		t.Errorf("forwarded request = %+v", gotReq)
	}

	var body struct {
		Products []map[string]any `json:"products"`
		Total    int64            `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 1 || len(body.Products) != 1 {
		t.Fatalf("body = %+v", body)
	}
	// snake_case contract + values.
	p := body.Products[0]
	if p["id"] != "p1" || p["price_cents"].(float64) != 1999 || p["available"].(float64) != 5 {
		t.Errorf("product DTO = %+v", p)
	}
}

// TestCreateProduct_AdminGate exercises the role gate on POST /products: an admin
// token creates the product (201, request forwarded), a customer token is
// forbidden (403), and an unauthenticated request is rejected before the role
// check (401). The fake user client maps the bearer token to a role so the same
// server serves all three cases.
func TestCreateProduct_AdminGate(t *testing.T) {
	var gotReq *productv1.CreateProductRequest
	pc := &fakeProductClient{
		createFn: func(in *productv1.CreateProductRequest) (*productv1.CreateProductResponse, error) {
			gotReq = in
			return &productv1.CreateProductResponse{
				Product: &productv1.Product{Id: "p9", Sku: in.GetSku(), Name: in.GetName(), PriceCents: in.GetPriceCents(), Available: in.GetInitialQuantity(), ImageUrl: in.GetImageUrl()},
			}, nil
		},
	}
	uc := &fakeUserClient{
		validateFn: func(in *userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
			role := "customer"
			if in.GetToken() == "admin-token" {
				role = "admin"
			}
			return &userv1.ValidateTokenResponse{Valid: true, UserId: "u1", Role: role}, nil
		},
	}
	h := gateway.NewHandler(uc, pc, &fakeCartClient{}, &fakeOrderClient{}, testMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(h.Router())
	t.Cleanup(ts.Close)

	const imageURL = "https://cdn.example.com/new.png"
	const body = `{"sku":"NEW-1","name":"New Thing","price_cents":1500,"initial_quantity":7,"image_url":"` + imageURL + `"}`

	post := func(token string) (int, []byte) {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/products", strings.NewReader(body))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b
	}

	status, respBody := post("admin-token")
	if status != http.StatusCreated {
		t.Errorf("admin: status = %d, want 201", status)
	}
	// The admin request reached the service with the body forwarded intact.
	if gotReq.GetSku() != "NEW-1" || gotReq.GetName() != "New Thing" || gotReq.GetPriceCents() != 1500 || gotReq.GetInitialQuantity() != 7 || gotReq.GetImageUrl() != imageURL {
		t.Errorf("forwarded create request = %+v", gotReq)
	}
	// image_url is part of the returned DTO contract.
	var dto map[string]any
	if err := json.Unmarshal(respBody, &dto); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if dto["image_url"] != imageURL {
		t.Errorf("response image_url = %v, want %q", dto["image_url"], imageURL)
	}

	if status, _ := post("customer-token"); status != http.StatusForbidden {
		t.Errorf("customer: status = %d, want 403", status)
	}
	if status, _ := post(""); status != http.StatusUnauthorized {
		t.Errorf("anonymous: status = %d, want 401", status)
	}
}

// TestUpdateProduct_AdminGate checks PATCH /products/{id}: an admin updates the
// product (200, id + fields forwarded), a customer is forbidden (403).
func TestUpdateProduct_AdminGate(t *testing.T) {
	var gotReq *productv1.UpdateProductRequest
	pc := &fakeProductClient{
		updateFn: func(in *productv1.UpdateProductRequest) (*productv1.UpdateProductResponse, error) {
			gotReq = in
			return &productv1.UpdateProductResponse{
				Product: &productv1.Product{Id: in.GetId(), Name: in.GetName(), PriceCents: in.GetPriceCents(), Available: in.GetQuantity()},
			}, nil
		},
	}
	uc := &fakeUserClient{
		validateFn: func(in *userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
			role := "customer"
			if in.GetToken() == "admin-token" {
				role = "admin"
			}
			return &userv1.ValidateTokenResponse{Valid: true, UserId: "u1", Role: role}, nil
		},
	}
	h := gateway.NewHandler(uc, pc, &fakeCartClient{}, &fakeOrderClient{}, testMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(h.Router())
	t.Cleanup(ts.Close)

	const body = `{"name":"Renamed","price_cents":2500,"quantity":12}`
	patch := func(token string) int {
		req, err := http.NewRequest(http.MethodPatch, ts.URL+"/products/p1", strings.NewReader(body))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PATCH: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode
	}

	if got := patch("admin-token"); got != http.StatusOK {
		t.Errorf("admin: status = %d, want 200", got)
	}
	if gotReq.GetId() != "p1" || gotReq.GetName() != "Renamed" || gotReq.GetPriceCents() != 2500 || gotReq.GetQuantity() != 12 {
		t.Errorf("forwarded update request = %+v", gotReq)
	}
	if got := patch("customer-token"); got != http.StatusForbidden {
		t.Errorf("customer: status = %d, want 403", got)
	}
}

// TestDeleteProduct_AdminGate checks DELETE /products/{id}: admin -> 204 (id
// forwarded), customer -> 403.
func TestDeleteProduct_AdminGate(t *testing.T) {
	var gotID string
	pc := &fakeProductClient{
		deleteFn: func(in *productv1.DeleteProductRequest) (*productv1.DeleteProductResponse, error) {
			gotID = in.GetId()
			return &productv1.DeleteProductResponse{}, nil
		},
	}
	uc := &fakeUserClient{
		validateFn: func(in *userv1.ValidateTokenRequest) (*userv1.ValidateTokenResponse, error) {
			role := "customer"
			if in.GetToken() == "admin-token" {
				role = "admin"
			}
			return &userv1.ValidateTokenResponse{Valid: true, UserId: "u1", Role: role}, nil
		},
	}
	h := gateway.NewHandler(uc, pc, &fakeCartClient{}, &fakeOrderClient{}, testMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(h.Router())
	t.Cleanup(ts.Close)

	del := func(token string) int {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/products/p1", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("DELETE: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		return resp.StatusCode
	}

	if got := del("admin-token"); got != http.StatusNoContent {
		t.Errorf("admin: status = %d, want 204", got)
	}
	if gotID != "p1" {
		t.Errorf("forwarded id = %q, want p1", gotID)
	}
	if got := del("customer-token"); got != http.StatusForbidden {
		t.Errorf("customer: status = %d, want 403", got)
	}
}

func TestGetProduct_OKAndNotFound(t *testing.T) {
	fake := &fakeProductClient{
		getFn: func(in *productv1.GetProductRequest) (*productv1.GetProductResponse, error) {
			if in.GetProductId() == "missing" {
				return nil, status.Error(codes.NotFound, "product not found")
			}
			return &productv1.GetProductResponse{Product: &productv1.Product{Id: in.GetProductId(), Name: "Widget"}}, nil
		},
	}
	ts := newProductTestServer(t, fake)

	ok, err := http.Get(ts.URL + "/products/p1")
	if err != nil {
		t.Fatalf("GET existing: %v", err)
	}
	defer func() { _ = ok.Body.Close() }()
	if ok.StatusCode != http.StatusOK {
		t.Errorf("GET existing: status = %d, want 200", ok.StatusCode)
	}

	nf, err := http.Get(ts.URL + "/products/missing")
	if err != nil {
		t.Fatalf("GET missing: %v", err)
	}
	defer func() { _ = nf.Body.Close() }()
	if nf.StatusCode != http.StatusNotFound {
		t.Errorf("GET missing: status = %d, want 404", nf.StatusCode)
	}
}
