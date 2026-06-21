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

	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
)

// Handler holds the gateway's dependencies. It depends on the generated
// UserServiceClient INTERFACE, so tests can inject a fake and real code injects
// a gRPC-backed client — same seam as everywhere else in this codebase.
type Handler struct {
	users userv1.UserServiceClient
	log   *slog.Logger
}

func NewHandler(users userv1.UserServiceClient, log *slog.Logger) *Handler {
	return &Handler{users: users, log: log}
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

	return r
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
