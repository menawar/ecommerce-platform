FROM golang:1.25-alpine AS builder
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/server ./services/product/cmd/productd
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/healthcheck ./pkg/cmd/healthcheck

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /bin/server /server
COPY --from=builder /bin/healthcheck /healthcheck
# Drop root: run as a non-root numeric UID (scratch has no /etc/passwd to name one).
USER 65532:65532
CMD ["/server"]
