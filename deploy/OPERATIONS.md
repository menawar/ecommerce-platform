# Operations runbook

Operating the platform once it's deployed to Kubernetes (see
`deploy/helm/ecommerce`). Assumes the kube-prometheus-stack (Prometheus
Operator + Grafana) is installed for the monitoring resources to take effect.

## Observability

- **Metrics** — every service serves Prometheus metrics on its http port
  (`/metrics`). Enable scraping with `--set metrics.serviceMonitor.enabled=true`
  (set `metrics.serviceMonitor.labels` to match your Prometheus
  `serviceMonitorSelector`). Key series: `grpc_server_handled_total{method,code}`,
  `http_server_handled_total{method,pattern,code}`, and the `*_handling_seconds`
  histograms.
- **Dashboard** — `metrics.grafanaDashboard.enabled=true` ships a ConfigMap the
  Grafana sidecar auto-imports (gRPC QPS / error ratio / p99, gateway HTTP by
  status).
- **Alerts** — `metrics.prometheusRule.enabled=true` adds: `ServiceDown`,
  `HighGrpcErrorRate`, `HighHttp5xxRate`, `HighGrpcLatencyP99`.
- **Tracing** — OpenTelemetry → Jaeger; one trace spans gateway → services →
  payment, so a slow/failed checkout is followed end to end (start at the
  gateway span, follow `order.*`/`payment.*`).

## Incident response

1. Page fires (e.g. `HighHttp5xxRate`). Identify the `job` from the alert labels.
2. `kubectl -n ecommerce get pods` — `CrashLoopBackOff`/`NotReady`? `kubectl logs`
   (structured slog; grep the `request_id`/`trace_id`).
3. Find the trace in Jaeger by service + error; the failing span names the
   culprit RPC.
4. Mitigate: scale (`kubectl scale deploy/<svc> --replicas=N`) or roll back
   (below). The saga's compensation means a failed checkout releases stock and
   cancels — no partial orders.

## Rollback

Releases are versioned by image tag (CD pushes `:vX.Y.Z`). To roll back:

```bash
helm history ecommerce -n ecommerce
helm rollback ecommerce <REVISION> -n ecommerce --wait
```

The migration hook runs on upgrade; **migrations must be backward-compatible**
with the previous app version (expand-then-contract) so a rollback doesn't break
on schema it can't read.

## Backups & PITR

- **Production uses managed Postgres** (`infra.enabled=false`): rely on the
  provider's automated backups + point-in-time recovery; verify retention and
  run a **restore drill** quarterly.
- **In-cluster Postgres** (dev/kind only): not backed up — do not run production
  data here. If you must, schedule a `pg_dump` CronJob to object storage.
- Test restores, not just backups — an untested backup is a hope, not a plan.

## Scaling

- App services are stateless `Deployment`s — raise `replicas` or add a
  HorizontalPodAutoscaler keyed on CPU or `grpc_server_handling_seconds`.
- The **outbox poller** and the **notification consumer** are safe to run at >1
  replica: the outbox dedupes on `published_at` and consumers dedupe on
  `event_id` (at-least-once).
- Postgres/Redis/NATS scale via their managed offerings, not by bumping replicas
  on the in-cluster StatefulSets.

## Load test (pre-launch)

Drive the saga (register → cart → checkout) with a tool like `k6`/`vegeta`
against the gateway; watch the Grafana dashboard for p99 latency and error ratio,
and confirm **no overselling** under concurrent checkout of limited stock (the
mandatory invariant). Right-size requests/limits from the observed usage.
