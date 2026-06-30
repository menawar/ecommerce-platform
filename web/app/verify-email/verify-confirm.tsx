"use client";

// Client Component so the token is consumed only on an explicit click (a POST via
// useActionState), NOT during the Server Component's GET render. That keeps email
// security scanners / link-preview bots, which issue automated GETs, from spending
// the single-use token before the human confirms.
import { useActionState } from "react";
import Link from "next/link";

import { verifyEmailAction, type VerifyState } from "@/app/(auth)/actions";
import { ResendButton } from "./resend-button";

export function VerifyConfirm({ token, loggedIn }: { token: string; loggedIn: boolean }) {
  const [state, formAction, pending] = useActionState<VerifyState, FormData>(verifyEmailAction, {});

  if (state.status === "ok") {
    return (
      <>
        <h1 style={{ fontSize: 22, fontWeight: 800, margin: 0 }}>Email verified</h1>
        <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", margin: "10px 0 0" }}>
          Your email is confirmed — you can now place orders.
        </p>
        <Link
          href="/products"
          className="plt-btn-primary-lg"
          style={{ display: "block", textDecoration: "none", marginTop: 24 }}
        >
          Continue shopping
        </Link>
      </>
    );
  }

  if (state.status === "invalid") {
    return (
      <>
        <h1 style={{ fontSize: 22, fontWeight: 800, margin: 0 }}>Link expired or invalid</h1>
        <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", margin: "10px 0 0" }}>
          This verification link is no longer valid.{" "}
          {loggedIn ? "Request a fresh one below." : "Sign in to request a new link."}
        </p>
        {loggedIn ? (
          <ResendButton />
        ) : (
          <Link
            href="/login"
            className="plt-btn-primary-lg"
            style={{ display: "block", textDecoration: "none", marginTop: 24 }}
          >
            Sign in
          </Link>
        )}
      </>
    );
  }

  // Initial state: a confirm button. Nothing is consumed until this form is POSTed.
  return (
    <form action={formAction}>
      <input type="hidden" name="token" value={token} />
      <h1 style={{ fontSize: 22, fontWeight: 800, margin: 0 }}>Confirm your email</h1>
      <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", margin: "10px 0 0" }}>
        Click below to verify your email address and unlock checkout.
      </p>
      <button
        disabled={pending}
        className="plt-btn-primary-lg"
        style={{ width: "100%", marginTop: 24 }}
      >
        {pending ? "Verifying…" : "Verify my email"}
      </button>
    </form>
  );
}
