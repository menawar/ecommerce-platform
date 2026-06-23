---
name: new-service
description: Scaffold a new Go microservice in this repo following its established conventions (module, proto, migration, sqlc, gRPC server, daemon, Makefile targets, dev script). Use when adding a new service.
---

Scaffold a new service `<name>` matching the existing pattern (mirror `services/product` or `services/cart`). Do it as gated steps with tests, not one dump.

1. **Module:** `services/<name>/go.mod` — module `github.com/menawar/ecommerce-platform/services/<name>`, `go 1.25.0`, `replace` for `pkg` and `proto`. Then `go work use ./services/<name>`.
2. **Proto:** `proto/<name>/v1/<name>.proto` → `buf generate`. buf STANDARD lint needs a **unique response type per RPC** and a `Service`-suffixed service name.
3. **If it has a DB:**
   - `services/<name>/migrations/000001_*.{up,down}.sql` (reversible).
   - `services/<name>/sqlc.yaml` (engine postgresql, `sql_package: pgx/v5`, `emit_pointers_for_null_types: true`).
   - `services/<name>/internal/db/queries.sql`.
   - Makefile: add `<NAME>_DB_URL` + `<NAME>_MIGRATIONS` vars and `<name>-migrate-up/down` + `<name>-sqlc` targets (call `$(MIGRATE)`/`$(SQLC)` by absolute path); add to `.PHONY`.
   - `make <name>-migrate-up && make <name>-sqlc`.
4. **Server:** `internal/server/server.go` implementing the gRPC server (validate input, map domain errors to gRPC status codes, never leak internals); `internal/server/server_test.go` (bufconn + integration, **SKIP** when DB unavailable; use `miniredis` for Redis-backed services).
5. **Daemon:** `cmd/<name>d/main.go` — pgx pool / go-redis / NATS as needed, gRPC + HTTP (`/metrics`,`/healthz`) under one `errgroup`, `pkg/grpcmw` logging+recovery interceptors, graceful shutdown, **own ports** (next free gRPC 5005x / http 211x).
6. **Wire up:** `go mod tidy`, `make build`, add the binary to `scripts/dev-backend.sh`. If the gateway should expose it, add a client + routes there.

Honor the repo rules: database-per-service, idempotency on retryable ops, the cart/price trust boundary, and ship tests with the feature.
