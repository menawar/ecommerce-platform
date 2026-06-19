# Scalable E-Commerce Platform — Technical Specification (Go)

**Version:** 1.0
**Stack:** Go 1.23+, PostgreSQL, Redis, NATS JetStream, gRPC, Docker Compose
**Architecture:** Database-per-service microservices with an orchestrated order saga

---

## 1. Goals & Non-Goals

### Goals
- Build a working e-commerce backend as independently deployable Go microservices.
- Demonstrate real distributed-systems patterns: service boundaries, sync (gRPC) and async (events) communication, the saga pattern, idempotency, and the transactional outbox.
- Run the whole thing locally via Docker Compose, with observability (metrics, logs, traces).

### Non-Goals (explicitly out of scope for v1)
- A frontend UI. This is an API-only backend. Test via `curl`/Postman/integration tests.
- Kubernetes. Docker Compose is the target. K8s is a documented stretch goal only.
- Real money. Payment runs in **mock mode** by default; Stripe is an optional adapter.
- Multi-region, sharding, or anything that isn't needed to prove the patterns.

---

## 2. System Architecture

```
                    ┌──────────────┐
   Client (HTTP) ──▶│  API Gateway │  (REST/JSON in, gRPC out, JWT validation)
                    └──────┬───────┘
                           │ gRPC
        ┌──────────┬───────┼─────────┬──────────────┐
        ▼          ▼       ▼         ▼              ▼
   ┌────────┐ ┌─────────┐ ┌──────┐ ┌────────┐ ┌─────────┐
   │  User  │ │ Product │ │ Cart │ │ Order  │ │ Payment │
   │ Service│ │ Catalog │ │ Svc  │ │ Service│ │ Service │
   └───┬────┘ └────┬────┘ └──┬───┘ └───┬────┘ └────┬────┘
       │           │         │         │           │
       ▼           ▼         ▼         ▼           ▼
   [Postgres]  [Postgres] [Redis]  [Postgres]  [Postgres]
                                        │
                                        │ publishes/consumes events
                                        ▼
                              ┌──────────────────┐
                              │   NATS JetStream  │◀── Notification Service
                              └──────────────────┘        (consumer only)
```

### Communication rules
- **Client → Gateway:** REST/JSON over HTTP.
- **Gateway → services:** gRPC (synchronous request/response).
- **Service → service (commands/queries):** gRPC. Kept to a minimum — only the Order saga calls other services synchronously.
- **Service → service (facts that happened):** asynchronous **events** over NATS JetStream. Notification Service is purely an event consumer.
- **Database-per-service:** No service ever touches another service's database. Cross-service data is obtained via gRPC or events only. This is the single most important rule.

---

## 3. Technology Choices (decided, not optional)

| Concern | Choice | Why |
|---|---|---|
| Language | Go 1.23+ | Concurrency, static binaries, first-class gRPC |
| HTTP router (gateway) | `go-chi/chi` | Lightweight, idiomatic, middleware-friendly |
| RPC | `grpc-go` + Protocol Buffers | Type-safe contracts between services |
| Relational DB | PostgreSQL 16 | One instance, one database per service |
| DB driver | `jackc/pgx/v5` | Fast, modern Postgres driver |
| Type-safe queries | `sqlc` | Generates Go from SQL — no ORM magic, you see the queries |
| Migrations | `golang-migrate` | Versioned, repeatable schema migrations |
| Cart store / cache | Redis 7 (`redis/go-redis`) | Cart is ephemeral and read-heavy |
| Messaging | NATS JetStream | Lightweight, Go-native, durable streams |
| Auth | `golang-jwt/jwt/v5` + `bcrypt` | Stateless JWT access tokens |
| Config | env vars via `kelseyhightower/envconfig` | 12-factor, simple |
| Logging | stdlib `log/slog` (JSON handler) | Structured logs, no dependency |
| Metrics | `prometheus/client_golang` | Standard scrape endpoint |
| Tracing | OpenTelemetry + OTLP → Jaeger | See requests cross service boundaries |
| Containers | Docker + Docker Compose | Local orchestration |
| CI | GitHub Actions | Lint, test, build per service |

---

## 4. Repository Layout

Monorepo, one module per service plus a shared module. This keeps it manageable while preserving service independence.

```
ecommerce-platform/
├── go.work                     # Go workspace tying the modules together
├── docker-compose.yml
├── docker-compose.infra.yml    # postgres, redis, nats, jaeger, prometheus
├── Makefile
├── proto/                      # .proto contracts (source of truth for gRPC)
│   ├── user/v1/user.proto
│   ├── product/v1/product.proto
│   ├── cart/v1/cart.proto
│   ├── order/v1/order.proto
│   └── payment/v1/payment.proto
├── pkg/                        # shared module: cross-cutting, NO business logic
│   ├── auth/                   # JWT issue/validate
│   ├── events/                 # event envelope, NATS publisher/consumer helpers
│   ├── outbox/                 # transactional outbox helpers
│   ├── observability/          # slog, prometheus, otel setup
│   └── httputil/               # error responses, request ID middleware
├── services/
│   ├── gateway/
│   ├── user/
│   ├── product/
│   ├── cart/
│   ├── order/
│   ├── payment/
│   └── notification/
└── deploy/
    └── prometheus.yml
```

Each service directory follows the same internal shape:

```
services/<name>/
├── Dockerfile
├── go.mod
├── cmd/server/main.go          # wiring only
├── internal/
│   ├── config/                 # envconfig struct
│   ├── domain/                 # entities, business rules (no I/O)
│   ├── store/                  # postgres/redis impls + sqlc output
│   ├── service/                # application logic (use cases)
│   ├── transport/
│   │   ├── grpc/               # gRPC server handlers
│   │   └── events/             # event publishers + consumers
│   └── db/
│       ├── migrations/         # golang-migrate .sql files
│       └── queries/            # .sql files for sqlc
└── ...
```

---

## 5. Service Specifications

### 5.1 User Service

**Responsibility:** Registration, authentication, profile, JWT issuance.

**Database: `userdb`**
```sql
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         CITEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    full_name     TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'customer',  -- customer | admin
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**gRPC API (`user.v1.UserService`)**
| Method | Request | Response | Notes |
|---|---|---|---|
| `Register` | email, password, full_name | user_id | hashes password with bcrypt |
| `Login` | email, password | access_token, refresh_token | issues JWT; access TTL 15m, refresh 7d |
| `ValidateToken` | token | user_id, role, valid | called by Gateway on every protected request |
| `GetProfile` | user_id | user fields | |
| `UpdateProfile` | user_id, full_name | ok | |

**JWT claims:** `sub` (user_id), `role`, `exp`, `iat`, `jti`. Signed HS256 with a shared secret from env (`JWT_SECRET`). For a stretch goal, switch to RS256 so only User Service holds the private key.

**Events published:** `user.registered` (user_id, email) — consumed by Notification.

---

### 5.2 Product Catalog Service

**Responsibility:** Products, categories, inventory levels and reservations.

**Database: `productdb`**
```sql
CREATE TABLE categories (
    id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name  TEXT UNIQUE NOT NULL,
    slug  TEXT UNIQUE NOT NULL
);

CREATE TABLE products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sku         TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_cents BIGINT NOT NULL CHECK (price_cents >= 0),
    currency    TEXT NOT NULL DEFAULT 'NGN',
    category_id UUID REFERENCES categories(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE inventory (
    product_id   UUID PRIMARY KEY REFERENCES products(id),
    quantity     INT NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    reserved     INT NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    -- available = quantity - reserved
    version      INT NOT NULL DEFAULT 0   -- optimistic lock
);
```

**gRPC API (`product.v1.ProductService`)**
| Method | Request | Response | Notes |
|---|---|---|---|
| `CreateProduct` | sku, name, price, category | product | admin only |
| `GetProduct` | product_id | product + available qty | |
| `ListProducts` | page, page_size, category, search | products[], total | |
| `ReserveStock` | items[{product_id, qty}], reservation_id | ok / insufficient | **idempotent** on reservation_id; uses optimistic locking |
| `ReleaseStock` | reservation_id | ok | compensating action |
| `CommitStock` | reservation_id | ok | turns reservation into a real decrement |

**Critical detail — stock reservation:** `ReserveStock` must be atomic and idempotent. Use a single SQL `UPDATE ... WHERE quantity - reserved >= :qty AND version = :version` and check rows-affected. Store reservations in a `stock_reservations` table keyed by `reservation_id` so a retry with the same id is a no-op. This is what makes the order saga safe.

**Events published:** `product.stock_low` (when available < threshold) — optional, for notifications.

---

### 5.3 Cart Service

**Responsibility:** Per-user shopping cart. Ephemeral, fast, no strong durability needs.

**Store: Redis** (no Postgres). Key design:
```
cart:{user_id}                  -> Redis HASH  field=product_id  value=quantity
cart:{user_id}                  -> TTL 30 days, refreshed on write
```

**gRPC API (`cart.v1.CartService`)**
| Method | Request | Response |
|---|---|---|
| `GetCart` | user_id | items[{product_id, qty}] |
| `AddItem` | user_id, product_id, qty | cart |
| `UpdateItem` | user_id, product_id, qty | cart |
| `RemoveItem` | user_id, product_id | cart |
| `ClearCart` | user_id | ok |

**Note:** Cart stores only product_id + quantity. Prices and names are resolved at checkout time by Order Service calling Product Catalog — never trust a price stored in the cart. `ClearCart` is called by Order Service (via event consumption or gRPC) after a successful order.

---

### 5.4 Order Service (the saga orchestrator)

**Responsibility:** Turn a cart into a paid order, coordinating Product, Payment, and Cart. This is the hardest and most valuable service.

**Database: `orderdb`**
```sql
CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    status          TEXT NOT NULL,   -- see state machine below
    total_cents     BIGINT NOT NULL,
    currency        TEXT NOT NULL DEFAULT 'NGN',
    reservation_id  UUID NOT NULL,
    payment_id      UUID,
    idempotency_key TEXT UNIQUE,     -- dedupes "place order" requests
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE order_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES orders(id),
    product_id  UUID NOT NULL,
    name        TEXT NOT NULL,        -- snapshot at order time
    price_cents BIGINT NOT NULL,      -- snapshot at order time
    quantity    INT NOT NULL
);

-- Transactional outbox: events written in the SAME tx as state changes
CREATE TABLE outbox (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic       TEXT NOT NULL,
    payload     JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);
```

**Order state machine:**
```
PENDING ─▶ STOCK_RESERVED ─▶ PAYMENT_PENDING ─▶ PAID ─▶ CONFIRMED
   │              │                  │
   │              │                  └─▶ PAYMENT_FAILED ─▶ CANCELLED
   │              └─▶ (reservation fails) ─▶ CANCELLED
   └─▶ (validation fails) ─▶ CANCELLED
```

**The saga (orchestration), happy path:**
1. Receive `PlaceOrder(user_id, idempotency_key)`. If key already seen → return existing order (idempotent).
2. Fetch cart (gRPC → Cart) and product details + prices (gRPC → Product). Compute total. Snapshot item names/prices into `order_items`.
3. `INSERT order` as `PENDING` + write step in one tx.
4. Call `Product.ReserveStock(reservation_id = order_id)`. On success → `STOCK_RESERVED`. On failure → `CANCELLED`, done.
5. Call `Payment.CreatePayment(order_id, amount, idempotency_key)` → `PAYMENT_PENDING`.
6. On payment success → `Product.CommitStock(reservation_id)`, set `PAID`, write `order.paid` to outbox, clear cart. Then `CONFIRMED`.
7. On payment failure → `Product.ReleaseStock(reservation_id)` (compensating action), set `CANCELLED`, write `order.cancelled` to outbox.

**Why outbox:** You cannot atomically "update Postgres AND publish to NATS." So you write the event into the `outbox` table inside the same DB transaction as the state change, then a background goroutine polls unpublished rows and pushes them to NATS, marking `published_at`. This guarantees at-least-once delivery with no lost events. Consumers must be idempotent.

**gRPC API (`order.v1.OrderService`)**
| Method | Request | Response |
|---|---|---|
| `PlaceOrder` | user_id, idempotency_key | order_id, status |
| `GetOrder` | order_id | order + items |
| `ListOrders` | user_id, page | orders[] |
| `CancelOrder` | order_id | ok (only if not yet PAID) |

**Events published:** `order.paid`, `order.cancelled`, `order.confirmed`.

---

### 5.5 Payment Service

**Responsibility:** Process payments. Runs in **mock mode** by default; Stripe is a pluggable adapter.

**Database: `paymentdb`**
```sql
CREATE TABLE payments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID NOT NULL,
    amount_cents    BIGINT NOT NULL,
    currency        TEXT NOT NULL,
    status          TEXT NOT NULL,   -- pending | succeeded | failed
    provider        TEXT NOT NULL,   -- mock | stripe
    provider_ref    TEXT,
    idempotency_key TEXT UNIQUE NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**gRPC API (`payment.v1.PaymentService`)**
| Method | Request | Response | Notes |
|---|---|---|---|
| `CreatePayment` | order_id, amount, idempotency_key | payment_id, status | idempotent on key |
| `GetPayment` | payment_id | payment | |

**Provider interface (the key abstraction):**
```go
type Provider interface {
    Charge(ctx context.Context, amountCents int64, currency, ref string) (providerRef string, err error)
}
```
- `MockProvider`: succeeds unless amount ends in specific test digits (e.g. amount % 100 == 13 → fail), so you can deterministically test the failure/compensation path.
- `StripeProvider`: implemented later as a stretch goal using PaymentIntents.

**Events published:** `payment.succeeded`, `payment.failed`.

---

### 5.6 Notification Service

**Responsibility:** Consume events and send (mock) email/SMS. Pure consumer — exposes no gRPC business API, only a health endpoint.

**Consumes:** `user.registered`, `order.paid`, `order.cancelled`, `order.confirmed`.

**Behavior:** For v1, "sending" means a structured log line + a row in a `notifications` table (Postgres `notificationdb`) for inspection. Swap in SendGrid/Twilio adapters later behind a `Sender` interface, exactly like the payment Provider pattern.

```sql
CREATE TABLE notifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID,
    channel    TEXT NOT NULL,    -- email | sms
    template   TEXT NOT NULL,    -- order_confirmation, welcome, ...
    payload    JSONB NOT NULL,
    sent_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Idempotency:** Each event carries a unique `event_id`. Notification stores processed `event_id`s and skips duplicates (because outbox delivery is at-least-once).

---

### 5.7 API Gateway

**Responsibility:** Single HTTP entry point. Validates JWTs, routes to gRPC services, translates REST ↔ gRPC, applies rate limiting and request IDs.

**Why a custom Go gateway (vs Kong/Traefik) for this project:** Building it yourself teaches you the request lifecycle, auth middleware, and gRPC client management. Note in your README that in production you'd likely front this with Traefik/Kong; here it's intentional for learning.

**REST surface (examples):**
```
POST   /api/v1/auth/register        -> User.Register
POST   /api/v1/auth/login           -> User.Login
GET    /api/v1/products             -> Product.ListProducts
GET    /api/v1/products/{id}        -> Product.GetProduct
POST   /api/v1/products             -> Product.CreateProduct        (admin)
GET    /api/v1/cart                 -> Cart.GetCart                  (auth)
POST   /api/v1/cart/items           -> Cart.AddItem                 (auth)
PATCH  /api/v1/cart/items/{pid}     -> Cart.UpdateItem              (auth)
DELETE /api/v1/cart/items/{pid}     -> Cart.RemoveItem              (auth)
POST   /api/v1/orders               -> Order.PlaceOrder             (auth)
GET    /api/v1/orders               -> Order.ListOrders             (auth)
GET    /api/v1/orders/{id}          -> Order.GetOrder               (auth)
```

**Middleware chain:** request ID → structured logging → JWT validation (for protected routes, calls `User.ValidateToken` or validates locally with shared secret) → rate limit (token bucket per user/IP via Redis) → handler.

---

## 6. Cross-Cutting Concerns

**Idempotency.** Any state-changing operation that can be retried (`PlaceOrder`, `CreatePayment`, `ReserveStock`) takes an idempotency key and dedupes on it via a unique constraint. This is non-negotiable in distributed systems — networks retry.

**Configuration.** Every service reads config from env vars only (host, DB DSN, NATS URL, JWT secret, log level). No config files baked into images.

**Error handling.** gRPC handlers return proper `codes` (`NotFound`, `InvalidArgument`, `FailedPrecondition` for insufficient stock, etc.). The gateway maps gRPC codes → HTTP status codes in one place.

**Money.** Always integer `*_cents` (BIGINT). Never floats for currency. Default currency `NGN`.

**Observability.**
- *Logs:* JSON via `slog`, every line carries `request_id` and `service`.
- *Metrics:* each service exposes `/metrics`; Prometheus scrapes. Track request count, latency histogram, error rate.
- *Tracing:* OpenTelemetry context propagated through gRPC metadata so one trace spans gateway → order → product → payment in Jaeger.

**Health checks.** Every service exposes gRPC health (`grpc.health.v1`) and an HTTP `/healthz`. Docker Compose uses these for `depends_on: condition: service_healthy`.

---

## 7. Event Catalog

| Topic | Producer | Consumers | Payload (core fields) |
|---|---|---|---|
| `user.registered` | User | Notification | user_id, email, full_name |
| `order.paid` | Order | Notification | order_id, user_id, total_cents |
| `order.cancelled` | Order | Notification | order_id, user_id, reason |
| `order.confirmed` | Order | Notification | order_id, user_id |
| `payment.succeeded` | Payment | Order | payment_id, order_id |
| `payment.failed` | Payment | Order | payment_id, order_id, reason |

All events use a shared envelope: `{ event_id, topic, occurred_at, version, data }`.

---

## 8. Build Phases (confirmation-gated)

Each phase is independently runnable and verifiable. **Do not start a phase until the previous one's acceptance criteria pass.** This is the discipline that gets the project finished instead of abandoned.

### Phase 0 — Foundation
- Repo + `go.work` + Makefile + `docker-compose.infra.yml` (Postgres, Redis, NATS, Jaeger, Prometheus).
- `pkg/` skeleton: `observability` (slog + prometheus + otel), `httputil`, `auth`.
- One throwaway "hello" gRPC service to prove the toolchain end to end.
- **Acceptance:** `make up` brings infra healthy; hello service responds to a gRPC call; `/metrics` scraped by Prometheus.

### Phase 1 — User Service + Gateway (auth slice)
- User Service: Register, Login, ValidateToken with bcrypt + JWT.
- Gateway: `/auth/register`, `/auth/login`, JWT middleware.
- **Acceptance:** Register → Login returns a JWT → a protected dummy endpoint accepts it and rejects a bad token. Integration test green.

### Phase 2 — Product Catalog
- CRUD + list with pagination/search. Inventory table. `ReserveStock`/`ReleaseStock`/`CommitStock` with idempotency + optimistic locking.
- Gateway product routes.
- **Acceptance:** Create products; concurrent `ReserveStock` for the same item never oversells (write a test that fires N goroutines at limited stock).

### Phase 3 — Cart
- Redis-backed cart, all CRUD methods, TTL.
- **Acceptance:** Add/update/remove items reflect correctly; cart survives service restart (Redis persistence), expires after TTL.

### Phase 4 — Order + Payment (the saga)
- Payment Service with `MockProvider` (deterministic failure path).
- Order Service: full saga, state machine, transactional outbox + publisher goroutine.
- **Acceptance:** Happy path → order `CONFIRMED`, stock committed, cart cleared, `order.paid` event emitted. Forced payment failure → stock released, order `CANCELLED`. Replaying the same `idempotency_key` returns the same order, no double charge.

### Phase 5 — Notification + Events end to end
- NATS JetStream wiring; Notification consumes events idempotently.
- **Acceptance:** Registering a user and completing an order produce notification rows; replaying an event produces no duplicate.

### Phase 6 — Observability + Compose polish
- Full Prometheus dashboards (Grafana optional), Jaeger traces spanning the saga, structured logs with request IDs, health-gated `depends_on`.
- **Acceptance:** A single placed order shows one connected trace across gateway → order → product → payment in Jaeger.

### Phase 7 — CI/CD
- GitHub Actions: lint (`golangci-lint`), test, build images per service on push.
- **Acceptance:** PR runs the matrix and fails on a broken test.

### Stretch goals (only after 0–7 are solid)
- Real Stripe adapter (PaymentIntents + webhook for async confirmation).
- Service discovery (Consul) instead of static Compose DNS.
- Kubernetes manifests + horizontal pod autoscaling.
- Refresh-token rotation; RS256 JWTs.
- gRPC streaming for an order-status feed.

---

## 9. Definition of Done (v1)

The platform is "done" when, with `docker compose up`:
1. A user can register and log in.
2. An admin can create products with stock.
3. A user can browse products, build a cart, and place an order.
4. A successful order charges (mock) payment, decrements stock exactly once, clears the cart, and notifies the user.
5. A failed payment releases stock and cancels the order — no stock leaks, no double charges.
6. Every request is traceable across services and visible in metrics and logs.
7. CI runs lint + tests on every push.

---

## 10. Realistic Effort Estimate

For one developer building this part-time alongside other work, expect roughly: Phase 0–1 in the first week, Product + Cart in the second, the Order/Payment saga taking the longest (it's the conceptual core — budget 1.5–2 weeks), and observability/CI another week. Total: about **5–6 weeks** of steady, evening-paced work. The saga is where the real learning lives; don't rush it.
