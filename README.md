# E-Commerce Microservices Platform

A production-grade microservices e-commerce platform built in Go and Next.js, demonstrating distributed systems patterns like the Saga pattern, transactional outboxes, and idempotency.

## Architecture
- **Backend**: Go microservices (User, Product, Cart, Order, Payment, Notification) behind an API Gateway.
- **Frontend**: Next.js App Router (BFF pattern).
- **Communication**: gRPC for sync, NATS JetStream for async events.
- **Data**: Database-per-service (PostgreSQL), Redis for Cart.
- **Observability**: Prometheus metrics, Jaeger distributed tracing, structured logging.

## Quick Start
1. Start infrastructure: `make infra-up`
2. Run database migrations: `for d in user product payment order notification; do make ${d}-migrate-up; done`
3. Start backend services: `./scripts/dev-backend.sh`
4. Start frontend: `cd web && npm run dev`

Access the web client at [http://localhost:3000](http://localhost:3000).

## Infrastructure Details
Docker images are built using multi-stage builds and run `FROM scratch` with `CGO_ENABLED=0` for tiny, secure deployments.

See `CLAUDE.md` and `ecommerce-platform-spec.md` for full technical specifications.
