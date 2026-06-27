package gateway_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/menawar/ecommerce-platform/pkg/ratelimit"
	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	"github.com/menawar/ecommerce-platform/services/gateway/internal/gateway"
)

// TestRateLimit_429 proves the middleware caps requests: with a burst of 1, the
// second request from the same client (IP-keyed on the public /products route) is
// rejected with 429 + Retry-After.
func TestRateLimit_429(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	limiter := ratelimit.New(rdb, 1, 1) // 1 tok/s, burst 1
	pc := &fakeProductClient{
		listFn: func(*productv1.ListProductsRequest) (*productv1.ListProductsResponse, error) {
			return &productv1.ListProductsResponse{}, nil
		},
	}
	h := gateway.NewHandler(&fakeUserClient{}, pc, &fakeCartClient{}, &fakeOrderClient{},
		testMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil))).WithLimiter(limiter)

	ts := httptest.NewServer(h.Router())
	defer ts.Close()

	first, err := ts.Client().Get(ts.URL + "/products")
	if err != nil {
		t.Fatalf("first GET: %v", err)
	}
	_ = first.Body.Close()
	if first.StatusCode == http.StatusTooManyRequests {
		t.Fatal("first request should pass the limiter")
	}

	second, err := ts.Client().Get(ts.URL + "/products")
	if err != nil {
		t.Fatalf("second GET: %v", err)
	}
	_ = second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second request should be 429, got %d", second.StatusCode)
	}
	if second.Header.Get("Retry-After") == "" {
		t.Error("429 response should carry a Retry-After header")
	}
}

// TestRateLimit_DisabledWhenNil: with no limiter configured, requests are never
// throttled (the middleware is a no-op).
func TestRateLimit_DisabledWhenNil(t *testing.T) {
	pc := &fakeProductClient{
		listFn: func(*productv1.ListProductsRequest) (*productv1.ListProductsResponse, error) {
			return &productv1.ListProductsResponse{}, nil
		},
	}
	h := gateway.NewHandler(&fakeUserClient{}, pc, &fakeCartClient{}, &fakeOrderClient{},
		testMetrics(), slog.New(slog.NewTextHandler(io.Discard, nil))) // no WithLimiter

	ts := httptest.NewServer(h.Router())
	defer ts.Close()

	for i := 0; i < 5; i++ {
		resp, err := ts.Client().Get(ts.URL + "/products")
		if err != nil {
			t.Fatalf("GET %d: %v", i, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("request %d throttled with no limiter configured", i)
		}
	}
}
