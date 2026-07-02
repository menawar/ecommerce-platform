import type { HTMLAttributes } from "react";
import { cn } from "@/lib/cn";

export type BadgeVariant = "neutral" | "brand" | "gold" | "danger" | "success";

const variantStyles: Record<BadgeVariant, string> = {
  neutral: "border border-border bg-surface text-fg-muted",
  brand: "bg-brand-subtle text-brand",
  gold: "bg-gold/20 text-accent",
  danger: "bg-danger-bg text-danger",
  success: "bg-success-bg text-success",
};

// A small status/label pill (New, Bestseller, Low stock, order status, …).
export function Badge({
  variant = "neutral",
  className,
  ...props
}: HTMLAttributes<HTMLSpanElement> & { variant?: BadgeVariant }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-pill px-2.5 py-0.5 text-xs font-semibold",
        variantStyles[variant],
        className,
      )}
      {...props}
    />
  );
}
