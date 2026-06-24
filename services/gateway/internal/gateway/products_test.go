package gateway_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	"github.com/menawar/ecommerce-platform/services/gateway/internal/gateway"
)

// fakeProductClient stubs ProductServiceClient. Only List/Get are exercised by the
// gateway's public routes; the saga RPCs are present to satisfy the interface.
type fakeProductClient struct {
	listFn func(*productv1.ListProductsRequest) (*productv1.ListProductsResponse, error)
	getFn  func(*productv1.GetProductRequest) (*productv1.GetProductResponse, error)
}

var _ productv1.ProductServiceClient = (*fakeProductClient)(nil)

func (f *fakeProductClient) ListProducts(_ context.Context, in *productv1.ListProductsRequest, _ ...grpc.CallOption) (*productv1.ListProductsResponse, error) {
	return f.listFn(in)
}
func (f *fakeProductClient) GetProduct(_ context.Context, in *productv1.GetProductRequest, _ ...grpc.CallOption) (*productv1.GetProductResponse, error) {
	return f.getFn(in)
}
func (f *fakeProductClient) CreateProduct(context.Context, *productv1.CreateProductRequest, ...grpc.CallOption) (*productv1.CreateProductResponse, error) {
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

	resp, err := http.Get(ts.URL + "/products?page=2&page_size=5&q=wid&category_id=cat1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Params were forwarded to the gRPC request.
	if gotReq.GetPage() != 2 || gotReq.GetPageSize() != 5 || gotReq.GetSearch() != "wid" || gotReq.GetCategoryId() != "cat1" {
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
	defer ok.Body.Close()
	if ok.StatusCode != http.StatusOK {
		t.Errorf("GET existing: status = %d, want 200", ok.StatusCode)
	}

	nf, err := http.Get(ts.URL + "/products/missing")
	if err != nil {
		t.Fatalf("GET missing: %v", err)
	}
	defer nf.Body.Close()
	if nf.StatusCode != http.StatusNotFound {
		t.Errorf("GET missing: status = %d, want 404", nf.StatusCode)
	}
}
