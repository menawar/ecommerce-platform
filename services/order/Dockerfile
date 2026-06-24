FROM golang:1.25-alpine AS builder
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/server ./services/order/cmd/orderd
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/healthcheck ./pkg/cmd/healthcheck

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /bin/server /server
COPY --from=builder /bin/healthcheck /healthcheck
CMD ["/server"]
