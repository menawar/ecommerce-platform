"use client";

import { useActionState } from "react";

import { resetPasswordAction } from "@/app/(auth)/actions";

// The token rides in a hidden field and the password is only sent on submit (a
// POST), so nothing is consumed by a GET render or a link-scanner.
export function ResetForm({ token }: { token: string }) {
  const [state, formAction, pending] = useActionState(resetPasswordAction, {});

  return (
    <form action={formAction} style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      <input type="hidden" name="token" value={token} />
      <input
        name="password"
        type="password"
        required
        minLength={8}
        placeholder="New password (min 8 chars)"
        autoComplete="new-password"
        className="plt-input"
      />
      {state.error && (
        <div
          style={{
            fontSize: 13,
            color: "var(--plt-error)",
            background: "var(--plt-error-bg)",
            padding: "10px 12px",
            borderRadius: "var(--plt-radius-sm)",
          }}
        >
          {state.error}
        </div>
      )}
      <button disabled={pending} className="plt-btn-primary-lg" style={{ width: "100%", marginTop: 4 }}>
        {pending ? "Updating…" : "Set new password"}
      </button>
    </form>
  );
}
