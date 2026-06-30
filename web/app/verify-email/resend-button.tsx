"use client";

// A Client Component so it can track the pending + result state of the resend
// Server Action via useActionState. The action runs on the server (it reads the
// session cookie); this just drives the button and the confirmation message.
import { useActionState } from "react";

import { resendVerificationAction, type ResendState } from "@/app/(auth)/actions";

export function ResendButton() {
  const [state, formAction, pending] = useActionState<ResendState>(
    () => resendVerificationAction(),
    {},
  );

  return (
    <form action={formAction} style={{ marginTop: 16 }}>
      <button disabled={pending} className="plt-btn-outline" style={{ width: "100%" }}>
        {pending ? "Sending…" : "Resend verification email"}
      </button>
      {state.sent && (
        <p style={{ fontSize: 13, color: "var(--plt-green-deep)", marginTop: 10, textAlign: "center" }}>
          Sent — check your inbox for a fresh link.
        </p>
      )}
      {state.error && (
        <p style={{ fontSize: 13, color: "var(--plt-error)", marginTop: 10, textAlign: "center" }}>
          {state.error}
        </p>
      )}
    </form>
  );
}
