import { cn } from "@/lib/cn";

// A shimmering placeholder for loading states. Decorative, so aria-hidden.
export function Skeleton({ className }: { className?: string }) {
  return <div aria-hidden className={cn("animate-pulse rounded-md bg-surface", className)} />;
}
