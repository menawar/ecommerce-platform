// Money arrives as integer minor units (cents/kobo) — the same representation the
// DB uses. Format for display only; never do arithmetic on the major-unit float.
export function formatPrice(cents: number, currency: string): string {
  return new Intl.NumberFormat("en-NG", {
    style: "currency",
    currency: currency || "NGN",
  }).format(cents / 100);
}
