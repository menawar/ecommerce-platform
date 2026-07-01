import { cn } from "@/lib/cn";

// A read-only star rating. `value` is 0–5; `count` (optional) shows the number of
// reviews. Accessible: the visual stars are aria-hidden and a text label carries
// the meaning. (Real review data arrives in Phase E; this renders whatever value.)
export function Rating({
  value,
  count,
  className,
}: {
  value: number;
  count?: number;
  className?: string;
}) {
  const clamped = Math.max(0, Math.min(5, value));
  const full = Math.round(clamped);
  return (
    <span
      className={cn("inline-flex items-center gap-1", className)}
      aria-label={`Rated ${clamped.toFixed(1)} out of 5${count != null ? `, ${count} reviews` : ""}`}
    >
      <span aria-hidden className="text-star">
        {"★".repeat(full)}
        {"☆".repeat(5 - full)}
      </span>
      {count != null && <span className="text-xs text-fg-subtle">({count})</span>}
    </span>
  );
}
