import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/cn";
import { Spinner } from "./spinner";

export type ButtonVariant = "primary" | "gold" | "outline" | "ghost" | "danger";
export type ButtonSize = "sm" | "md" | "lg";

const base =
  "inline-flex items-center justify-center gap-2 rounded-pill font-bold transition-colors " +
  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/40 " +
  "disabled:cursor-not-allowed disabled:opacity-60";

const variantStyles: Record<ButtonVariant, string> = {
  primary: "bg-brand text-white hover:bg-brand-deep",
  gold: "bg-gold text-brand-deep hover:brightness-95",
  outline: "border border-border-strong bg-card text-fg hover:bg-surface",
  ghost: "text-fg hover:bg-surface",
  danger: "border border-danger text-danger hover:bg-danger-bg",
};

const sizeStyles: Record<ButtonSize, string> = {
  sm: "h-9 px-3.5 text-sm",
  md: "h-11 px-5 text-sm",
  lg: "h-12 px-6 text-base",
};

// buttonVariants returns just the class string, so a Link styled as a button can
// reuse the exact look: `<Link className={buttonVariants({ variant: "outline" })}>`.
export function buttonVariants({
  variant = "primary",
  size = "md",
}: { variant?: ButtonVariant; size?: ButtonSize } = {}): string {
  return cn(base, variantStyles[variant], sizeStyles[size]);
}

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  fullWidth?: boolean;
  loading?: boolean;
  children?: ReactNode;
}

export function Button({
  variant,
  size,
  fullWidth,
  loading,
  className,
  children,
  disabled,
  ...props
}: ButtonProps) {
  return (
    <button
      // Default to a non-submit button so a Button placed in a <form> doesn't submit
      // it by accident; submit/reset callers override via `type` in {...props}.
      type="button"
      className={cn(buttonVariants({ variant, size }), fullWidth && "w-full", className)}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...props}
    >
      {loading && <Spinner decorative className="h-4 w-4" />}
      {children}
    </button>
  );
}
