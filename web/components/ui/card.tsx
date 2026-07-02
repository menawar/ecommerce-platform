import type { ElementType, HTMLAttributes } from "react";
import { cn } from "@/lib/cn";

// The standard surface: rounded, bordered, subtly elevated. `padded` (default) adds
// comfortable inner spacing; turn it off when the content manages its own. `as` lets
// it render a semantic element (e.g. <aside>, <section>) instead of a <div>.
export function Card({
  as: Comp = "div",
  padded = true,
  className,
  ...props
}: HTMLAttributes<HTMLElement> & { as?: ElementType; padded?: boolean }) {
  return (
    <Comp
      className={cn("rounded-xl border border-border bg-card shadow-card", padded && "p-5", className)}
      {...props}
    />
  );
}
