"use client";

// A Client Component because it needs interactivity: useActionState tracks the
// pending state and the error returned by the Server Action between submissions.
// The action itself still runs on the server — we just drive the UI around it.
import { useActionState } from "react";
import Link from "next/link";

import type { AuthState } from "./actions";

export function AuthForm({
  action,
  mode,
}: {
  // The Server Action is passed in from the (server) page and invoked here.
  action: (prev: AuthState, formData: FormData) => Promise<AuthState>;
  mode: "login" | "register";
}) {
  const [state, formAction, pending] = useActionState(action, {});
  const isRegister = mode === "register";

  return (
    <form
      action={formAction}
      style={{
        marginTop: 24,
        display: "flex",
        flexDirection: "column",
        gap: 12,
      }}
    >
      {isRegister && (
        <input
          name="full_name"
          required
          placeholder="Full name"
          className="plt-input"
        />
      )}
      <input
        name="email"
        type="email"
        required
        placeholder="Email"
        autoComplete="email"
        className="plt-input"
      />
      <input
        name="password"
        type="password"
        required
        minLength={8}
        placeholder="Password (min 8 chars)"
        autoComplete={isRegister ? "new-password" : "current-password"}
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

      {!isRegister && (
        <div style={{ textAlign: "right", marginTop: -4 }}>
          <Link href="/forgot-password" style={{ fontSize: 13, color: "var(--plt-terracotta)", fontWeight: 600 }}>
            Forgot password?
          </Link>
        </div>
      )}

      <button
        disabled={pending}
        className="plt-btn-primary-lg"
        style={{ width: "100%", marginTop: 4 }}
      >
        {pending ? "Please wait…" : isRegister ? "Create account" : "Sign in"}
      </button>

      <p
        style={{
          fontSize: 13,
          color: "var(--plt-text-secondary)",
          textAlign: "center",
        }}
      >
        {isRegister ? (
          <>
            Already have an account?{" "}
            <Link
              href="/login"
              style={{
                color: "var(--plt-terracotta)",
                fontWeight: 600,
              }}
            >
              Sign in
            </Link>
          </>
        ) : (
          <>
            New here?{" "}
            <Link
              href="/register"
              style={{
                color: "var(--plt-terracotta)",
                fontWeight: 600,
              }}
            >
              Create an account
            </Link>
          </>
        )}
      </p>
    </form>
  );
}
