# E-Commerce Microservices Platform

Go microservices + a Next.js storefront, built phase-by-phase per `ecommerce-platform-spec.md`.
The conceptual core is the **order saga** with a **transactional outbox** and **idempotency**.

## Architecture
- **Services (database-per-service):** User, Product, Cart, Order (the saga orchestrator), Payment, Notification — behind an **API Gateway**.
- **Sync** comms: gRPC. **Async** comms: domain events over **NATS JetStream**, emitted via a **transactional outbox** (events written in the same DB tx as the state change; a poller publishes them).
- **Storage:** Postgres per service (one physical instance, logical DB each), Redis for the cart.
- **Frontend** (`web/`): Next.js App Router **BFF** — the Next *server* calls the gateway server-to-server; the JWT lives in an **httpOnly cookie**; no CORS needed.

## Workspace & modules
- `go.work` workspace. Modules: **`pkg`** (shared: auth, events, outbox, grpcmw, observability, postgres), **`proto`** (buf-generated), **`services/<name>`**.
- Intra-repo deps use `replace` directives (because `go mod tidy` ignores `go.work`).
- Toolchain lives in `$(go env GOPATH)/bin` — put it on PATH. **NOTE:** a system `migrate` (python) shadows golang-migrate, so the Makefile invokes `migrate`/`sqlc` by absolute path.

## Common commands
- **Run the stack:** `GATEWAY_HTTP_ADDR=:8090 ./scripts/dev-backend.sh` (gateway on :8090 here because :8080 is taken on this host; matches `web/.env.local`). Web: `cd web && npm run dev` → http://localhost:3000. (See `/dev`.)
- **Migrate all DBs:** `make infra-up && for d in user product payment order notification; do make ${d}-migrate-up; done` (See `/db-migrate`.)
- **Regenerate code:** `buf generate` (protos); `make <svc>-sqlc` (queries). Don't hand-edit generated files. (See `/gen`.)
- **Build all:** `make build`. **Test:** `go test -race ./...` per module; web: `cd web && npm run build && npx tsc --noEmit && npm run lint`.
- **Saga e2e:** see `/smoke`.

## Ports
| Service | gRPC | http (metrics/health) |
|---|---|---|
| user | 50051 | 2112 |
| product | 50052 | 2113 |
| cart | 50053 | 2114 |
| payment | 50054 | 2115 |
| order | 50055 | 2116 |
| notification | — (pure consumer) | 2117 |
| gateway | — | :8080 (use :8090 here) |

Infra: Postgres host `:5433`→5432, Redis `:6379`, NATS `:4222` (monitor `:8222`), Jaeger `:16686`, Prometheus host `:9091`.

## Conventions
- **Branches:** `feat/<phase>-<service>-<desc>`; frontend `feat/<phase>-web-<desc>`. **Conventional Commits.** **No Claude co-author** in commit messages.
- **Tests ship with every feature** (`go test -race`). DB integration tests **SKIP** (not fail) when the DB is unreachable. Mandatory: saga **compensation** (payment decline → stock released → CANCELLED) and **concurrent stock reservation** (N goroutines, never oversells).
- **Idempotency** on every retryable op (`PlaceOrder`, `CreatePayment`, `ReserveStock`) via a unique key.
- **Database-per-service:** no service reads another's tables — cross-service data only via gRPC or events.
- **Trust boundary:** the cart stores only `product_id` + `quantity` (never prices); prices are resolved at checkout from the Product service.
- **At-least-once events** → consumers must be **idempotent** (dedup on `event_id`).
- **Payment is mock** (`MockProvider`: `amount_cents % 100 == 13` → declined). No real card form (PCI is offloaded to Stripe in a real build).
- Run **`/code-review`** on the diff before merging each step's PR.

## Generated code (do not hand-edit)
- `proto/**/*.pb.go`, `proto/**/*_grpc.pb.go` — from `.proto` via `buf generate`.
- `services/**/internal/db/{db,models,queries.sql}.go` — from `migrations/` + `queries.sql` via `sqlc`.
