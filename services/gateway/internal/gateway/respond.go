package gateway

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// writeJSON sets the content type, status, and encodes body. Centralizing this
// means every response is consistent and we never forget the header.
func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// decodeJSON reads a JSON body into dst with two guards: a 1MiB cap (a client
// can't OOM us with an endless body) and DisallowUnknownFields (a typo'd field
// name is a 400, not silently ignored).
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// writeGRPCError maps an error from an upstream gRPC call onto an HTTP response.
// This is the REST<->gRPC error contract: the service speaks status codes, the
// gateway speaks HTTP codes. 5xx causes are logged but never echoed to the
// client (no leaking internal detail).
func (h *Handler) writeGRPCError(w http.ResponseWriter, r *http.Request, err error) {
	st := status.Convert(err) // turns any error into a *status.Status (Unknown if not one)
	httpCode := httpStatusFromGRPC(st.Code())

	msg := st.Message()
	if httpCode >= 500 {
		h.log.ErrorContext(r.Context(), "upstream error", "grpc_code", st.Code().String(), "err", err)
		msg = "internal error"
	}
	writeError(w, httpCode, msg)
}

// httpStatusFromGRPC translates the gRPC status codes our services use into the
// closest HTTP status. Only the codes we actually emit are enumerated; anything
// else collapses to 500.
func httpStatusFromGRPC(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument:
		return http.StatusBadRequest // 400
	case codes.Unauthenticated:
		return http.StatusUnauthorized // 401
	case codes.PermissionDenied:
		return http.StatusForbidden // 403
	case codes.NotFound:
		return http.StatusNotFound // 404
	case codes.AlreadyExists:
		return http.StatusConflict // 409
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout // 504
	case codes.Unavailable:
		return http.StatusServiceUnavailable // 503
	default:
		return http.StatusInternalServerError // 500
	}
}
