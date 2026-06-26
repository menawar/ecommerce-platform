// parsePriceToCents converts a user-entered major-unit price (e.g. "19.99") into
// integer minor units (1999). Money is integer cents/kobo everywhere downstream —
// the DB, the proto, the gateway — so this is the ONE place a major→minor
// conversion (and its rounding) happens. Returns null for empty, non-numeric, or
// negative input so the caller can reject it instead of sending garbage.
export function parsePriceToCents(input: string): number | null {
  const trimmed = input.trim();
  if (trimmed === "") return null;
  const value = Number(trimmed);
  if (!Number.isFinite(value) || value < 0) return null;
  return Math.round(value * 100);
}
