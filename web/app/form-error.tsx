import type { CSSProperties } from "react";

// InlineFormError is the small red error banner shown inside forms (driven by a
// Server Action's returned state). Extracted so the auth, address, shipping, and
// checkout forms share one styling — previously this block was copy-pasted in each.
// Renders nothing when there's no message, so callers can pass state.error directly.
export function InlineFormError({ message, style }: { message?: string; style?: CSSProperties }) {
  if (!message) return null;
  return (
    <div
      style={{
        fontSize: 13,
        color: "var(--plt-error)",
        background: "var(--plt-error-bg)",
        padding: "10px 12px",
        borderRadius: "var(--plt-radius-sm)",
        ...style,
      }}
    >
      {message}
    </div>
  );
}
