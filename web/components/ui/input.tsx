import type { InputHTMLAttributes, SelectHTMLAttributes, TextareaHTMLAttributes } from "react";
import { cn } from "@/lib/cn";

// Shared control styling: full-width, rounded, brand focus ring, danger border when
// invalid. `invalid` also sets aria-invalid so assistive tech announces the error.
const controlBase =
  "w-full rounded-md border bg-card text-sm text-fg placeholder:text-fg-subtle transition-colors " +
  "focus:border-brand focus:outline-none focus:ring-2 focus:ring-brand/25 " +
  "disabled:cursor-not-allowed disabled:opacity-60";

const borderFor = (invalid?: boolean) => (invalid ? "border-danger" : "border-border-strong");

export function Input({
  className,
  invalid,
  ...props
}: InputHTMLAttributes<HTMLInputElement> & { invalid?: boolean }) {
  return (
    <input
      className={cn(controlBase, "h-11 px-3.5", borderFor(invalid), className)}
      aria-invalid={invalid || undefined}
      {...props}
    />
  );
}

export function Textarea({
  className,
  invalid,
  ...props
}: TextareaHTMLAttributes<HTMLTextAreaElement> & { invalid?: boolean }) {
  return (
    <textarea
      className={cn(controlBase, "min-h-24 px-3.5 py-2.5", borderFor(invalid), className)}
      aria-invalid={invalid || undefined}
      {...props}
    />
  );
}

export function Select({
  className,
  invalid,
  ...props
}: SelectHTMLAttributes<HTMLSelectElement> & { invalid?: boolean }) {
  return (
    <select
      className={cn(controlBase, "h-11 px-3", borderFor(invalid), className)}
      aria-invalid={invalid || undefined}
      {...props}
    />
  );
}
