#!/usr/bin/env bash
# Run the backend services the web app needs (User, Product, Gateway) for local
# development. Ctrl-C stops all of them together.
#
# Prereqs (run once): make infra-up && make user-migrate-up && make product-migrate-up
#
# Ports: the gateway HTTP port defaults to :8080. If :8080 is taken on your host
# (as in some sandboxes), start with GATEWAY_HTTP_ADDR=:8090 and set
# web/.env.local -> GATEWAY_URL=http://localhost:8090 to match.
set -euo pipefail
cd "$(dirname "$0")/.."

export JWT_SECRET="${JWT_SECRET:-dev-insecure-secret}"
export GATEWAY_HTTP_ADDR="${GATEWAY_HTTP_ADDR:-:8080}"

# Deliver notification emails (verification link, order updates) to the Mailpit
# catcher started by `make infra-up` — view them at http://localhost:8025. Without
# this, NOTIFY_SENDER defaults to "log" and emails are only logged, never sent.
export NOTIFY_SENDER="${NOTIFY_SENDER:-smtp}"
export SMTP_ADDR="${SMTP_ADDR:-localhost:1025}"
export EMAIL_FROM="${EMAIL_FROM:-Plateau <no-reply@plateau.example>}"

echo "Building service binaries..."
mkdir -p bin
go build -o bin/userd ./services/user/cmd/userd
go build -o bin/productd ./services/product/cmd/productd
go build -o bin/cartd ./services/cart/cmd/cartd
go build -o bin/paymentd ./services/payment/cmd/paymentd
go build -o bin/orderd ./services/order/cmd/orderd
go build -o bin/notificationd ./services/notification/cmd/notificationd
go build -o bin/gatewayd ./services/gateway/cmd/gatewayd

echo "Starting user + product + cart + payment + order + notification + gateway (gateway at ${GATEWAY_HTTP_ADDR}). Ctrl-C to stop."
echo "Emails (verification link, order updates) → Mailpit inbox at http://localhost:8025"
# kill the whole process group on exit so no service is left orphaned (the same
# signal issue we hit with `go run`/`next start` children).
trap 'echo; echo "stopping services..."; kill 0' EXIT

./bin/userd &
./bin/productd &
./bin/cartd &
./bin/paymentd &
./bin/orderd &
./bin/notificationd &
./bin/gatewayd &
wait
