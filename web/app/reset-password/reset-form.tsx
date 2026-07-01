"use client";

import { useActionState } from "react";

import { InlineFormError } from "@/app/form-error";

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
      <InlineFormError message={state.error} />
      <button disabled={pending} className="plt-btn-primary-lg" style={{ width: "100%", marginTop: 4 }}>
        {pending ? "Updating…" : "Set new password"}
      </button>
    </form>
  );
}
