# Market-Readiness Plan

**Project:** E-Commerce Microservices Platform
**Date:** 2026-06-26
**Branch reviewed:** `feat/7-completion`
**Scope of this document:** Honest assessment of current state + a phased roadmap to take the project from a finished learning build (spec Phases 0–7) to a production-ready, market-launchable e-commerce platform.

---

## 1. Where the project actually stands

The spec's v1 (Phases 0–7) is **essentially complete** — and it is a genuinely solid distributed-systems backend.

**Implemented and working:**
- All 7 services + gateway: saga orchestration (`services/order/internal/saga`), transactional outbox, idempotency keys, optimistic-locked stock reservation (`services/product/internal/inventory`), NATS JetStream events, idempotent notification consumer.
- Cross-cutting: OpenTelemetry → Jaeger tracing, Prometheus metrics, `slog`, gRPC + HTTP health checks, refresh tokens issued, request IDs propagated.
- CI (`.github/workflows/ci.yml`), `golangci-lint`, multi-stage scratch Docker images, full `docker-compose.yml`.
- **Frontend** (beyond the spec's original non-goals): Next.js 16 / React 19 App Router BFF with auth, products, cart, checkout, orders, account pages — recently redesigned ("Plateau"), with ~220 web tests.

**The core problem:** the spec was explicitly a *learning project*. Its stated non-goals — *no frontend, no real money, no Kubernetes, no multi-region* — are exactly the things "ready for market" requires. The remaining gap is therefore not bug-fixing; it is a different class of work: real payments, security hardening, production operations, and the commerce features a learning saga intentionally skips.

---

## 2. Concrete gaps found in review

| Gap | Evidence | Severity for market |
|---|---|---|
| **No real payments** | Only `MockProvider` (`amount % 100 == 13` → fail) in `services/payment/internal/provider` | Blocker |
| **No admin surface** | `CreateProduct` exists in proto but no gateway route, no admin UI; `role=admin` defined but unused | Blocker (no way to load a catalog) |
| **No rate limiting** | Spec'd Redis token-bucket never implemented in gateway | High (security) |
| **No refresh / logout flow** | Refresh token issued but no `/auth/refresh` or `/auth/logout` route — 15m access sessions silently die | High |
| **No email verification / password reset** | Not present in user service | High |
| **No product images** | No object storage / image fields wired to UI | High (it is a store) |
| **No fulfillment** | Order lifecycle ends at CONFIRMED — no shipping, address, tax, SHIPPED/DELIVERED, refunds | High |
| **No real notifications** | Notification writes a DB row + log line; no SendGrid/Twilio | Medium |
| **No prod deploy / IaC / secrets mgmt** | Docker Compose only; JWT secret via raw env; no TLS, no managed data stores | Blocker |
| **No legal/compliance** | No ToS, privacy policy, GDPR delete/export, cookie consent | Blocker to launch |
| **Uncommitted / stray artifacts** | Working tree has unstaged redesign + `web/app/footer.tsx`, a checked-in `healthcheck` binary, and a stray `web/E-commerce platform frontend design (2)/` folder | Housekeeping |

---

## 3. Roadmap (continuing the spec's phase numbering)

The remaining work is grouped into 8 phases, keeping the project's confirmation-gated, test-with-every-feature discipline. Each phase has an acceptance criterion in the same style as the original spec.

### Phase 8 — Commerce completeness (catalog & checkout for real)
- Admin: gateway `POST/PATCH/DELETE /products` behind `role=admin` middleware + admin UI for catalog & inventory management.
- Product images: image URLs on products → object storage (S3 / Cloudflare R2) + CDN; upload flow in admin.
- Categories browse, search, pagination (already in DB layer) surfaced as filtering/sort in the UI.
- **Acceptance:** an admin can create a product with an image and stock; a customer sees it and completes a purchase end-to-end, no `curl`.

### Phase 9 — Real payments (Stripe)
- `StripeProvider` behind the existing `Provider` interface; PaymentIntents + **webhook** for async confirmation feeding back into the saga via events.
- Keep `MockProvider` for tests/CI. Money stays integer cents.
- **Acceptance:** a test-mode Stripe charge confirms an order via webhook; a decline still releases stock + cancels the order (existing compensation test extended).

### Phase 10 — Auth & account hardening
- `/auth/refresh` (rotation) + `/auth/logout` (cookie clear / refresh revocation), email verification, password reset, optionally RS256 JWTs.
- **Acceptance:** an expired access token transparently refreshes; logout invalidates the session; unverified users are gated.

### Phase 11 — Fulfillment & order lifecycle
- Shipping addresses, shipping methods, tax calculation; extend the state machine CONFIRMED → SHIPPED → DELIVERED; refunds/cancellations + Stripe refund.
- **Acceptance:** an order carries address + shipping cost in its total; an admin can mark it shipped; a refund releases funds + emits events.

### Phase 12 — Security & abuse hardening
- Redis token-bucket rate limiting (as originally spec'd), security headers, CSRF on BFF mutations, an input-validation pass, secrets via a manager (not raw env), dependency/container scanning in CI, `/security-review` gate.
- **Acceptance:** rate limits enforced; an OWASP-style checklist passes; secrets are not baked into images.

### Phase 13 — Real notifications & comms
- SendGrid/Twilio behind the existing `Sender` interface; transactional templates (welcome, order confirmation, shipped, password reset); dead-letter + retry for events.
- **Acceptance:** a real email arrives on order placement; a failed send is retried, not lost.

### Phase 14 — Production deployment & ops
- Kubernetes (or managed cloud) + Terraform; managed Postgres/Redis/NATS; TLS/ingress; container registry; CD pipeline (build → scan → deploy, blue-green); Grafana dashboards + alerting + SLOs; backups/PITR; load test.
- **Acceptance:** deploy from a git tag; one trace spans the saga in production; a killed pod self-heals; a restore-from-backup drill passes.

### Phase 15 — Launch readiness
- Legal (ToS, privacy policy, GDPR data-delete/export, cookie consent), PCI SAQ-A confirmation (offloaded to Stripe), SEO/accessibility/analytics, Sentry error tracking, runbooks.
- **Acceptance:** launch checklist signed off.

---

## 4. Recommended sequencing

The **critical path to "can actually transact"** is **Phase 8 → 9 → 12 → 14**. Fulfillment (11), real comms (13), and account polish (10) can run in parallel or slot in once a real order can be taken.

**Start with Phase 8 (admin + images)** — without it there is no catalog to sell, and it is the cheapest unblock.

**Housekeeping before any phase work:** commit or discard the current branch's redesign, and remove the checked-in `healthcheck` binary and the stray `web/E-commerce platform frontend design (2)/` folder from the working tree.

---

## 5. Open decisions that fork the plan

1. **Payments** — real Stripe (Phase 9 as scoped), or stay mock-only for now and treat this as a portfolio/demo "market ready"?
2. **Deployment target** — Kubernetes (the spec's stretch goal), a managed PaaS (Render/Fly/Railway), or a specific cloud (AWS/GCP)? This drives Phase 14 entirely.
