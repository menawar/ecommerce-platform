package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestHTTPMetricsMiddleware proves the middleware records the correct labels for
// a success and a not-found (unmatched) request. The key assertion is that the
// route PATTERN — not the raw URL — ends up in the label, preventing cardinality
// explosion from path parameters.
func TestHTTPMetricsMiddleware(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewHTTPMetrics(reg, "test")

	// Build a chi router with our middleware and one parameterised route.
	r := chi.NewRouter()
	r.Use(Middleware(m))
	r.Get("/products/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/orders", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	// --- matched GET with a path parameter ---
	req := httptest.NewRequest(http.MethodGet, "/products/abc123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /products/abc123 status = %d, want 200", rec.Code)
	}
	// The label should be the PATTERN "/products/{id}", NOT "/products/abc123".
	got := testutil.ToFloat64(m.handled.WithLabelValues("GET", "/products/{id}", "200"))
	if got != 1 {
		t.Errorf("handled{GET, /products/{id}, 200} = %v, want 1", got)
	}

	// --- matched POST ---
	req = httptest.NewRequest(http.MethodPost, "/orders", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /orders status = %d, want 201", rec.Code)
	}
	got = testutil.ToFloat64(m.handled.WithLabelValues("POST", "/orders", "201"))
	if got != 1 {
		t.Errorf("handled{POST, /orders, 201} = %v, want 1", got)
	}

	// --- unmatched path → 404, pattern = "unmatched" ---
	req = httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /nonexistent status = %d, want 404", rec.Code)
	}
	got = testutil.ToFloat64(m.handled.WithLabelValues("GET", "unmatched", "404"))
	if got != 1 {
		t.Errorf("handled{GET, unmatched, 404} = %v, want 1", got)
	}

	// Duration histogram should have observations for each method+pattern pair.
	// 3 requests = 3 observations across the series.
	if count := testutil.CollectAndCount(m.duration); count < 2 {
		t.Errorf("duration series count = %d, want at least 2 (matched + unmatched patterns)", count)
	}
}
