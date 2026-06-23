---
name: gen
description: Regenerate all generated code for the e-commerce platform — buf protobuf/gRPC stubs and sqlc query code — after editing a .proto or queries.sql. Use when proto/SQL changed or generated code looks stale.
---

Regenerate generated code from the repo root (ensure `$(go env GOPATH)/bin` is on PATH).

1. Protobuf / gRPC (all services share the `proto` module):
   ```
   buf lint && buf generate
   ```
   Remember: buf STANDARD lint requires a **unique response type per RPC** and a **`Service` suffix** on service names.
2. sqlc per service that has SQL:
   ```
   make product-sqlc user-sqlc payment-sqlc order-sqlc notification-sqlc
   ```
3. Confirm everything compiles: `make build`. Re-tidy any module whose imports changed: `(cd services/<name> && go mod tidy)`.

Commit the regenerated files together with the `.proto` / `queries.sql` change that drove them. Do NOT hand-edit `*.pb.go` or `internal/db/*.go`.
