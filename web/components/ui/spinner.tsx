import { cn } from "@/lib/cn";

// A loading spinner. Inherits color from `currentColor`. By default it's a live
// status region; pass `decorative` when the busy state is already announced by the
// container (e.g. inside a Button with aria-busy) to avoid a double announcement.
export function Spinner({ className, decorative }: { className?: string; decorative?: boolean }) {
  const a11y = decorative
    ? { "aria-hidden": true as const }
    : { role: "status", "aria-label": "Loading" };
  return (
    <span
      {...a11y}
      className={cn(
        "inline-block h-5 w-5 animate-spin rounded-full border-2 border-current border-t-transparent",
        className,
      )}
    />
  );
}
