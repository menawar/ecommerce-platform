// Package gateway is the HTTP edge of the platform. It exposes a REST/JSON API
// to the outside world and translates each call into a gRPC request to an
// internal service. It owns NO business logic — it decodes JSON, calls a
// service, maps the result back to HTTP. That thinness is the point: all rules
// live in the services, the gateway just adapts protocols.
package gateway

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	cartv1 "github.com/menawar/ecommerce-platform/proto/cart/v1"
	productv1 "github.com/menawar/ecommerce-platform/proto/product/v1"
	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
)

// Handler holds the gateway's dependencies — one generated client INTERFACE per
// backing service, so tests inject fakes and real code injects gRPC-backed
// clients. As the platform grows, the gateway fans out to more services from here.
type Handler struct {
	users    userv1.UserServiceClient
	products productv1.ProductServiceClient
	carts    cartv1.CartServiceClient
	log      *slog.Logger
}

func NewHandler(
	users userv1.UserServiceClient,
	products productv1.ProductServiceClient,
	carts cartv1.CartServiceClient,
	log *slog.Logger,
) *Handler {
	return &Handler{users: users, products: products, carts: carts, log: log}
}

// Router builds the middleware chain and routes. Middleware run top-to-bottom on
// the way IN and unwind on the way out, wrapping every handler — that's the
// "chain": RequestID tags the request, Recoverer turns a handler panic into a
// 500 instead of crashing the server, and requestLogger emits one structured
// line per request.
func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(h.requestLogger)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Post("/auth/register", h.register)
	r.Post("/auth/login", h.login)

	// Catalog browsing is public — anyone can shop before logging in.
	r.Get("/products", h.listProducts)
	r.Get("/products/{id}", h.getProduct)

	// Protected routes: a nested group with its own middleware. requireAuth runs
	// only for routes inside this group, leaving /auth/*, /products, /healthz public.
	r.Group(func(pr chi.Router) {
		pr.Use(h.requireAuth)
		pr.Get("/me", h.me)

		// Cart is per-user: every handler reads the user_id from the validated
		// token (Identity), never from the request — so one user can't touch
		// another's cart.
		pr.Get("/cart", h.getCart)
		pr.Post("/cart/items", h.addCartItem)
		pr.Put("/cart/items/{productID}", h.updateCartItem)
		pr.Delete("/cart/items/{productID}", h.removeCartItem)
	})

	return r
}

// me is a protected dummy endpoint that proves auth works: it returns the caller
// identity that requireAuth extracted from the token. The Phase 1 acceptance
// uses it as the "protected endpoint that accepts a good token, rejects a bad one".
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFrom(r.Context())
	if !ok {
		// Unreachable if requireAuth ran; defensive against a future routing slip.
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"user_id": id.UserID, "role": id.Role})
}

// requestLogger is a middleware: it takes the next handler and returns a handler
// that wraps it. The func(http.Handler) http.Handler shape is the entire
// middleware contract — chi, and net/http itself, know nothing more than that.
func (h *Handler) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// WrapResponseWriter lets us read back the status code the handler wrote,
		// which a bare http.ResponseWriter doesn't expose.
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		h.log.InfoContext(r.Context(), "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.users.Register(r.Context(), &userv1.RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
		FullName: req.FullName,
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"user_id": resp.GetUserId()})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.users.Login(r.Context(), &userv1.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		h.writeGRPCError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  resp.GetAccessToken(),
		"refresh_token": resp.GetRefreshToken(),
		"expires_at":    resp.GetExpiresAt(),
	})
}
