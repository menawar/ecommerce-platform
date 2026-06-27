# ---- Stage 1: Build ----
# golang:alpine gives us git + the Go toolchain. Everything below happens in
# this throwaway layer — none of it ships to production.
FROM golang:1.25-alpine AS builder

# Install CA certs and timezone data NOW so we can copy them into scratch later.
# Alpine's golang image doesn't ship tzdata by default.
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the full repo context (filtered by .dockerignore). The go.work workspace
# needs all modules visible so cross-module replace directives resolve.
COPY . .

# CGO_ENABLED=0 produces a fully static binary: no libc dependency, no dynamic
# linker needed. This is what lets us run on scratch (an empty filesystem).
# -ldflags="-s -w" strips the symbol table and DWARF debug info, cutting ~30%
# off binary size with no runtime effect.
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/server ./services/user/cmd/userd
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/healthcheck ./pkg/cmd/healthcheck

# ---- Stage 2: Runtime ----
# scratch is literally an empty filesystem — 0 bytes. Our static binary is the
# ONLY thing that runs. No shell, no package manager, no attack surface.
FROM scratch

# TLS: Go's crypto/tls looks for CA root certificates at well-known paths.
# Without these, any outbound HTTPS call (Stripe, OTEL collector, webhooks)
# fails with "x509: certificate signed by unknown authority".
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Timezone data: time.LoadLocation("Africa/Lagos") needs the IANA tz database.
# Without this, any timezone-aware code falls back to UTC silently or errors.
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# The service binary and our custom healthcheck (replaces wget, which doesn't
# exist in scratch).
COPY --from=builder /bin/server /server
COPY --from=builder /bin/healthcheck /healthcheck

# Drop root: run as a non-root numeric UID (scratch has no /etc/passwd to name one).
USER 65532:65532
CMD ["/server"]
