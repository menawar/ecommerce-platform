import type { HTMLAttributes } from "react";
import { cn } from "@/lib/cn";

// The standard surface: rounded, bordered, subtly elevated. `padded` (default) adds
// comfortable inner spacing; turn it off when the content manages its own.
export function Card({
  padded = true,
  className,
  ...props
}: HTMLAttributes<HTMLDivElement> & { padded?: boolean }) {
  return (
    <div
      className={cn("rounded-xl border border-border bg-card shadow-card", padded && "p-5", className)}
      {...props}
    />
  );
}
