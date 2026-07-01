import { cloneElement, isValidElement, type ReactElement, type ReactNode } from "react";
import { cn } from "@/lib/cn";
import { Label } from "./label";

type ControlProps = { "aria-describedby"?: string; "aria-invalid"?: boolean };

// Field composes a labelled form control with an optional hint or error. It wires
// accessibility for you: the label associates via htmlFor/id, and the hint/error is
// linked to the control with aria-describedby (+ aria-invalid on error) by cloning
// the single control child — so screen readers announce the message and error state.
export function Field({
  label,
  htmlFor,
  hint,
  error,
  children,
  className,
}: {
  label?: ReactNode;
  htmlFor?: string;
  hint?: string;
  error?: string;
  children: ReactNode;
  className?: string;
}) {
  const message = error ?? hint;
  const messageId = htmlFor && message ? `${htmlFor}-message` : undefined;

  const control =
    isValidElement(children) && (messageId || error)
      ? cloneElement(children as ReactElement<ControlProps>, {
          "aria-invalid": error ? true : (children as ReactElement<ControlProps>).props["aria-invalid"],
          "aria-describedby":
            [(children as ReactElement<ControlProps>).props["aria-describedby"], messageId]
              .filter(Boolean)
              .join(" ") || undefined,
        })
      : children;

  return (
    <div className={cn("flex flex-col", className)}>
      {label && <Label htmlFor={htmlFor}>{label}</Label>}
      {control}
      {message && (
        <p id={messageId} className={cn("mt-1 text-xs", error ? "text-danger" : "text-fg-subtle")}>
          {message}
        </p>
      )}
    </div>
  );
}
