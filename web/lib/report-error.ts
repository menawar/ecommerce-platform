// Isomorphic error-reporting seam for the Next.js layer. Structured logging is the
// default sink (picked up by the platform's log pipeline / Grafana alerts from
// Phase 14), so no Sentry account is needed to build, test, or run. When a Sentry
// DSN is configured, real reporting plugs in at the marked point below. The Go
// services report their own errors via sentry-go (see pkg/observability.Reporter).
type ErrorContext = Record<string, string | number | undefined>;

export function reportError(error: unknown, context: ErrorContext = {}): void {
  const message = error instanceof Error ? error.message : String(error);
  const digest = error instanceof Error ? (error as { digest?: string }).digest : undefined;
  console.error("[error-report]", JSON.stringify({ message, digest, ...context }));

  const dsn = process.env.SENTRY_DSN ?? process.env.NEXT_PUBLIC_SENTRY_DSN;
  if (!dsn) return;
  // DROP-IN (config-gated): with a DSN set, forward to Sentry here, e.g.
  //   import("@sentry/nextjs").then((S) => S.captureException(error));
  // Kept as a documented one-step so the SDK + its build integration aren't
  // required for the seam to exist and be tested.
}
