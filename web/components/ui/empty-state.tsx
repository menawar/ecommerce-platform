import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

// A centered "nothing here yet" panel — empty cart, no orders, no search results.
export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
  as: Heading = "h2",
}: {
  icon?: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
  className?: string;
  // Heading level — use "h1" when the empty state is a page's primary heading (e.g.
  // an otherwise-empty cart page) so the document outline stays correct.
  as?: "h1" | "h2" | "h3";
}) {
  return (
    <div className={cn("flex flex-col items-center px-6 py-14 text-center", className)}>
      {icon && <div className="mb-3 text-4xl text-fg-subtle">{icon}</div>}
      <Heading className="text-lg font-bold text-fg">{title}</Heading>
      {description && <p className="mt-1 max-w-sm text-sm text-fg-muted">{description}</p>}
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}
