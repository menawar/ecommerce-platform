---
name: dev
description: Build and run the full local backend stack (all 7 Go services) for the e-commerce platform, and report health. Use when the user wants to start/run the services for manual testing or to drive the web app.
---

Bring up the local backend for the e-commerce platform.

1. Ensure infra is up and every DB is migrated:
   ```
   make infra-up
   for d in user product payment order notification; do make ${d}-migrate-up; done
   ```
2. Run all backend services in the background (Ctrl-C / process-group kill stops all). Gateway listens on **:8090** here because :8080 is taken on this host (matches `web/.env.local`):
   ```
   export PATH="$PATH:$(go env GOPATH)/bin"
   GATEWAY_HTTP_ADDR=:8090 ./scripts/dev-backend.sh
   ```
   This runs user, product, cart, payment, order, notification, gateway.
3. Verify health: gateway `curl :8090/products`; service health ports 2112–2117 (`curl :211X/healthz`).
4. For the web app, tell the user to run in a second shell: `cd web && npm run dev` → http://localhost:3000

Notes:
- Use the **built binaries** (the script builds them), not `go run` — `go run`/`next start` don't forward SIGTERM cleanly and orphan children.
- If a port is already bound, find/kill the owner by port before restarting.
- Report which services came up and any that failed (with the log tail).
