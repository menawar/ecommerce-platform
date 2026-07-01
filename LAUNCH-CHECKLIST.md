# Launch checklist

The go/no-go gate for taking the platform live. Work top to bottom; every box
must be ticked (or consciously waived in **Sign-off**). Deep-dive references:
`deploy/OPERATIONS.md` (runbook), `deploy/helm/ecommerce` (chart),
`ecommerce-platform-spec.md` (design), `MARKET-READINESS-PLAN.md` (phases).

## 1. Infrastructure & deploy
- [ ] Kubernetes cluster reachable; `helm template | kubeconform -strict` passes.
- [ ] Managed Postgres, Redis, NATS JetStream provisioned (or in-cluster
      `infra.enabled=true` for a small launch); connection strings set as secrets.
- [ ] All DB migrations applied (migrate Job runs pre-app: user, product, payment,
      order, notification).
- [ ] Ingress + TLS live (cert-manager); gateway at `/`, Paystack webhook at
      `/webhooks/paystack`, `/metrics` NOT publicly exposed.
- [ ] CD pipeline green on the release tag (build + Trivy scan + push all images).

## 2. Secrets & config (no placeholders in prod)
- [ ] `JWT_SECRET` — strong, unique, not the committed dev value.
- [ ] Per-service DB URLs, `NATS_URL`, `REDIS_URL`.
- [ ] `PAYSTACK_SECRET_KEY` + webhook secret; `PAYMENT_PROVIDER=paystack`.
- [ ] `NOTIFY_SENDER=smtp` + real `SMTP_ADDR`/`EMAIL_FROM` (a real relay, not Mailpit).
- [ ] `SENTRY_DSN` + `ENVIRONMENT=production` on services and web (error tracking on).
- [ ] `NEXT_PUBLIC_SITE_URL` = the real origin (drives canonical URLs, sitemap, OG).
- [ ] `WEB_BASE_URL` = the real origin (emailed verify/reset links).

## 3. Payments (PCI)
- [ ] **PCI DSS SAQ-A** applies and is satisfied: the platform never sees, stores,
      or transmits cardholder data — payment is fully offloaded to Paystack
      (redirect / hosted fields), and the DB stores only a provider reference +
      amount/status (see `services/payment`; the trust boundary is enforced by the
      cart storing only `product_id`+`quantity`). Confirm no card data reaches our
      logs or storage.
- [ ] Live Paystack keys; webhook URL registered and signature-verified; a real
      test charge + refund succeeds end to end.

## 4. Observability & ops
- [ ] ServiceMonitors + PrometheusRule alerts enabled
      (`ServiceDown`, `HighGrpcErrorRate`, `HighHttp5xxRate`, `HighGrpcLatencyP99`).
- [ ] Grafana dashboard imported; Jaeger receiving traces.
- [ ] Sentry receiving events (trigger a test error; confirm `trace_id` links to Jaeger).
- [ ] On-call knows the runbook (`deploy/OPERATIONS.md`): incident response,
      rollback, backups/PITR.
- [ ] Postgres backups + PITR verified by a test restore.

## 5. Legal & data protection
- [ ] Terms (`/terms`) and Privacy Policy (`/privacy`) reviewed and current.
- [ ] Cookie notice shows; only the strictly-necessary session cookie is used.
- [ ] Data rights work end to end: self-service **export** (`/account` → JSON) and
      **deletion** (anonymises the user + cascades `user.deleted` → order snapshots).
- [ ] Privacy contact (`privacy@…`) monitored.

## 6. SEO & accessibility
- [ ] `robots.txt` allows the public store, disallows authed/token pages.
- [ ] `sitemap.xml` lists static routes + all products.
- [ ] Per-page metadata + OpenGraph; product JSON-LD validates (Rich Results test).
- [ ] Keyboard pass: skip-to-content works; forms are labelled; images have alt text.

## 7. Security
- [ ] Rate limiting on (per-IP public, per-user authed).
- [ ] Email verification gating checkout; password reset revokes sessions.
- [ ] `govulncheck` + Trivy clean in CI; workloads run non-root, read-only rootfs.
- [ ] No secrets in the image or git; TLS everywhere; CORS not needed (BFF).

## 8. Pre-launch smoke (against staging)
- [ ] Register → verify email → browse → add to cart → checkout (Paystack) →
      order CONFIRMED → ship → deliver → refund.
- [ ] Saga compensation: a declined payment releases stock and CANCELS.
- [ ] `web`: `npm run build && npx tsc --noEmit && npm run lint` green; `go test -race`
      green per module.

---

## Sign-off

| Area | Owner | Status | Date |
|---|---|---|---|
| Infrastructure & deploy | | ☐ | |
| Secrets & config | | ☐ | |
| Payments (PCI SAQ-A) | | ☐ | |
| Observability & ops | | ☐ | |
| Legal & data protection | | ☐ | |
| SEO & accessibility | | ☐ | |
| Security | | ☐ | |
| Smoke tests | | ☐ | |

**Go / No-go:** ______________  **Approver:** ______________  **Date:** __________
