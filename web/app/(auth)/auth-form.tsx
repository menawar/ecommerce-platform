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
    <form action={formAction} className="mt-6 flex flex-col gap-3">
      {isRegister && (
        <input
          name="full_name"
          required
          placeholder="Full name"
          className="rounded-md border border-zinc-300 px-3 py-2"
        />
      )}
      <input
        name="email"
        type="email"
        required
        placeholder="Email"
        autoComplete="email"
        className="rounded-md border border-zinc-300 px-3 py-2"
      />
      <input
        name="password"
        type="password"
        required
        minLength={8}
        placeholder="Password (min 8 chars)"
        autoComplete={isRegister ? "new-password" : "current-password"}
        className="rounded-md border border-zinc-300 px-3 py-2"
      />

      {state.error && <p className="text-sm text-red-600">{state.error}</p>}

      <button
        disabled={pending}
        className="rounded-md bg-foreground px-4 py-2 font-medium text-background disabled:opacity-60"
      >
        {pending ? "Please wait…" : isRegister ? "Create account" : "Sign in"}
      </button>

      <p className="text-sm text-zinc-500">
        {isRegister ? (
          <>
            Already have an account?{" "}
            <Link href="/login" className="underline">
              Sign in
            </Link>
          </>
        ) : (
          <>
            New here?{" "}
            <Link href="/register" className="underline">
              Create an account
            </Link>
          </>
        )}
      </p>
    </form>
  );
}
