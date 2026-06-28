# ecommerce Helm chart

Deploys the application tier — the API **gateway** plus the 7 services
(user, product, cart, payment, order, notification) — as Kubernetes
Deployments + Services rendered from a single template.

Backing infra (Postgres, Redis, NATS, MinIO) ships **in-cluster** behind
`infra.enabled` (default `true`) as StatefulSets + PVCs, so a `kind`/`minikube`
cluster is self-contained. In **production** set `infra.enabled=false` and point
`config`/`secrets` at managed data stores — the apps use the same hostnames, so
nothing else changes.

> **Migrations** are not yet automated by this chart. After the DBs are up, run
> them as a one-time step (port-forward the `postgres` service and use the
> existing `make <svc>-migrate-up` targets). A migration Helm hook Job lands with
> the CD increment (where the migrator image is built alongside the services).

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

## Ingress + TLS

`ingress.enabled=true` exposes the public surface via one Ingress (needs an
ingress controller + cert-manager):

- `/webhooks/paystack` → the **payment** service (Paystack calls this from the
  internet). Routing by *path* keeps `/metrics` on the same port private.
- `/` → the **gateway** REST API. In a fully in-cluster topology (web BFF also in
  the cluster), drop this rule and keep the gateway ClusterIP-internal.

```bash
helm upgrade --install rel deploy/helm/ecommerce \
  --set ingress.enabled=true \
  --set ingress.host=api.yourstore.com \
  --set ingress.tls.clusterIssuer=letsencrypt-prod
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
