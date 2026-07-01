// Package grpcmw holds shared gRPC server interceptors — the server-side
// equivalent of HTTP middleware. An interceptor wraps every RPC so cross-cutting
// concerns (logging, panic recovery, later: tracing, metrics, auth) live in one
// place instead of being copy-pasted into every handler.
package grpcmw

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryLogging logs one structured line per unary RPC: the full method, the
// resulting status code, and how long it took. It is the OUTER interceptor so it
// observes the final outcome even after inner interceptors (like recovery) have
// converted a panic into an error.
//
// The signature is the whole interceptor contract: receive the request plus a
// `handler` (the rest of the chain ending in the real method), call it, and
// return its result — doing work before and/or after.
func UnaryLogging(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err) // OK when err == nil

		log.LogAttrs(ctx, levelForCode(code), "grpc request",
			slog.String("method", info.FullMethod),
			slog.String("code", code.String()),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		)
		return resp, err
	}
}

// UnaryRecovery converts a panic in a handler into a clean codes.Internal error
// (logging the stack) instead of letting it crash the whole server process. It
// is the INNER interceptor, closest to the handler, so it catches panics from
// the actual method.
//
// Note the NAMED return values (resp, err): the deferred closure can only change
// what the function returns if the returns are named — that's how recover() turns
// a panic into a returned error.
func UnaryRecovery(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorContext(ctx, "recovered from panic in grpc handler",
					"method", info.FullMethod,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

// errorReporter is the sliver of observability.Reporter this package needs, kept as
// a local interface so grpcmw doesn't import observability (avoids an import cycle
// and keeps the dependency explicit).
type errorReporter interface {
	Report(ctx context.Context, err error, tags map[string]string)
}

// UnaryErrorReporting sends server-fault RPC failures (the same codes logged at
// Error level) to the reporter — the external error-tracking sink. Client-fault
// codes (bad input, unauthenticated, not-found) are NOT reported: they're normal,
// and would drown real bugs in noise. It sits OUTSIDE recovery so a panic that
// recovery turns into codes.Internal is reported too.
func UnaryErrorReporting(r errorReporter) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err != nil && isServerFault(status.Code(err)) {
			r.Report(ctx, err, map[string]string{"method": info.FullMethod})
		}
		return resp, err
	}
}

// isServerFault reports whether a code means "our bug" (worth capturing) vs a
// client-caused outcome. It's the Error-level set from levelForCode MINUS
// Unavailable — a transient dependency blip logs at Error but shouldn't spam the
// error tracker as if it were a code defect.
func isServerFault(code codes.Code) bool {
	switch code {
	case codes.Internal, codes.Unknown, codes.DataLoss:
		return true
	default:
		return false
	}
}

// levelForCode picks a log level from the RPC outcome: success is Info, our-fault
// codes are Error, and client-fault codes (bad input, unauthenticated) are Warn —
// so an alert on Error noise isn't triggered by users typing wrong passwords.
func levelForCode(code codes.Code) slog.Level {
	switch code {
	case codes.OK:
		return slog.LevelInfo
	case codes.Internal, codes.Unknown, codes.DataLoss, codes.Unavailable:
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}
