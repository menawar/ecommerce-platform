"use client";

import { useActionState } from "react";

import { InlineFormError } from "@/app/form-error";
import Link from "next/link";

import { forgotPasswordAction } from "@/app/(auth)/actions";

export function ForgotForm() {
  const [state, formAction, pending] = useActionState(forgotPasswordAction, {});

  if (state.sent) {
    return (
      <div style={{ textAlign: "center" }}>
        <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", margin: 0 }}>
          If an account exists for that email, we’ve sent a password-reset link.
          Check your inbox.
        </p>
        <Link
          href="/login"
          className="plt-btn-primary-lg"
          style={{ display: "block", textDecoration: "none", marginTop: 24 }}
        >
          Back to sign in
        </Link>
      </div>
    );
  }

  return (
    <form action={formAction} style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      <input name="email" type="email" required placeholder="Email" autoComplete="email" className="plt-input" />
      <InlineFormError message={state.error} />
      <button disabled={pending} className="plt-btn-primary-lg" style={{ width: "100%", marginTop: 4 }}>
        {pending ? "Sending…" : "Send reset link"}
      </button>
      <p style={{ fontSize: 13, color: "var(--plt-text-secondary)", textAlign: "center" }}>
        Remembered it?{" "}
        <Link href="/login" style={{ color: "var(--plt-terracotta)", fontWeight: 600 }}>
          Sign in
        </Link>
      </p>
    </form>
  );
}
