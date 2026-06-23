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

echo "Building service binaries..."
mkdir -p bin
go build -o bin/userd ./services/user/cmd/userd
go build -o bin/productd ./services/product/cmd/productd
go build -o bin/cartd ./services/cart/cmd/cartd
go build -o bin/paymentd ./services/payment/cmd/paymentd
go build -o bin/orderd ./services/order/cmd/orderd
go build -o bin/gatewayd ./services/gateway/cmd/gatewayd

echo "Starting userd + productd + cartd + paymentd + orderd + gatewayd (gateway at ${GATEWAY_HTTP_ADDR}). Ctrl-C to stop."
# kill the whole process group on exit so no service is left orphaned (the same
# signal issue we hit with `go run`/`next start` children).
trap 'echo; echo "stopping services..."; kill 0' EXIT

./bin/userd &
./bin/productd &
./bin/cartd &
./bin/paymentd &
./bin/orderd &
./bin/gatewayd &
wait
