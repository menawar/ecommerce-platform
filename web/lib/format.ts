// Money arrives as integer minor units (cents/kobo) — the same representation the
// DB uses. Format for display only; never do arithmetic on the major-unit float.
export function formatPrice(cents: number, currency: string): string {
  return new Intl.NumberFormat("en-NG", {
    style: "currency",
    currency: currency || "NGN",
  }).format(cents / 100);
}

// The gateway sends created_at as epoch SECONDS (Go's time.Unix()), but JS Date
// expects milliseconds — hence × 1000. Forgetting this silently dates everything
// to 1970.
export function formatDate(epochSeconds: number): string {
  return new Intl.DateTimeFormat("en-NG", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(epochSeconds * 1000));
}
