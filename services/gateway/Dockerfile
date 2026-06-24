FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/server ./services/gateway/cmd/gatewayd

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /bin/server /server
CMD ["/server"]
