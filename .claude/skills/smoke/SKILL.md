---
name: smoke
description: Run the full end-to-end order-saga smoke test through the running services — register, cart, place order (happy + forced decline), and verify CONFIRMED/CANCELLED, stock, cart, and notifications. Use to confirm the whole stack works together.
---

With infra up and all services running (use `/dev` first), run a saga smoke test. Use `grpcurl` on the service gRPC ports or the gateway HTTP API on :8090. Always set `export PATH="$PATH:$(go env GOPATH)/bin"`.

**Happy path → CONFIRMED:**
1. Register a user (gateway `POST /auth/register` + `/auth/login` for a token, or user gRPC :50051 `Register`). Capture the user_id.
2. Create a product priced so the total is NOT `% 100 == 13` (product gRPC :50052 `CreateProduct`, e.g. price 3000, qty 10). Capture product id.
3. Add it to the cart (cart gRPC :50053 `AddItem`, or gateway `POST /cart/items` with the Bearer token).
4. Place the order (order gRPC :50055 `PlaceOrder`, or gateway `POST /orders` with an `Idempotency-Key` header). Expect **CONFIRMED**.
5. Verify: product `available` dropped (committed); cart cleared (`redis-cli HGETALL cart:<user_id>` empty).
6. Replay the same idempotency key → **same order_id** (no double charge).

**Forced failure → CANCELLED:**
1. Product priced so total `% 100 == 13` (e.g. 1313, qty 1) → add to cart → `PlaceOrder` → expect **CANCELLED**.
2. Verify product `available` is UNCHANGED (stock released, no leak).

**Events → notifications:**
- After register + a confirmed order, check: `docker exec ecommerce-infra-postgres-1 psql -U ecommerce -d notificationdb -tAc "SELECT template FROM notifications ORDER BY sent_at;"` → expect welcome, payment_received, order_confirmation.
- The outbox poller runs every ~1s, so wait ~2–3s before checking notifications.

Report each outcome (status, stock, cart, notifications). Note: the EVENTS JetStream stream is persistent, so counts accumulate across runs unless purged.
