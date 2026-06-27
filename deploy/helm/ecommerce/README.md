# ecommerce Helm chart

Deploys the application tier — the API **gateway** plus the 7 services
(user, product, cart, payment, order, notification) — as Kubernetes
Deployments + Services rendered from a single template.

This chart deploys **only the apps**. Its backing infra (Postgres, Redis, NATS,
MinIO, Jaeger) must already be reachable at the hostnames in `values.yaml`
(`postgres`, `redis`, `nats`, `jaeger`). In-cluster infra is a later increment;
in production you point `config`/`secrets` at managed data stores.

## How it's wired

- One `templates/deployment.yaml` ranges over `.Values.services`. Each gets a
  non-root securityContext (UID 65532, read-only rootfs, all caps dropped — a
  tmpfs `/tmp` is mounted), `/healthz` readiness+liveness probes, and resource
  requests/limits.
- A shared **ConfigMap** holds the service-to-service connect topology
  (`user:50051`, …, resolved by cluster DNS). Each pod overrides **only its own
  bind address** in `env` (a container `env` entry takes precedence over
  `envFrom`), so the `user` pod binds `:50051` while everyone else dials `user:50051`.
- A **Secret** holds `JWT_SECRET`, the per-service `*_DB_URL`s, and Paystack keys.

## Quick start (local, e.g. kind/minikube)

```bash
# Render + schema-check without a cluster:
helm template rel deploy/helm/ecommerce | kubeconform -strict -summary

# Install (assumes infra + images are present in the cluster/registry):
helm install rel deploy/helm/ecommerce
```

## Production

**Never deploy the committed placeholder secrets.** Override them out-of-band:

```bash
helm upgrade --install rel deploy/helm/ecommerce \
  --set image.tag=v1.2.3 \
  --set-string secrets.JWT_SECRET="$(openssl rand -base64 48)" \
  --set-string secrets.PAYSTACK_SECRET_KEY="sk_live_..." \
  --set-string secrets.PAYSTACK_WEBHOOK_SECRET="sk_live_..." \
  --set config.PAYMENT_PROVIDER=paystack
```

Better still, manage secrets with the External Secrets Operator or
sealed-secrets so nothing sensitive touches `values.yaml`. `APP_ENV=production`
makes the user service refuse to boot with a missing/weak `JWT_SECRET`.
