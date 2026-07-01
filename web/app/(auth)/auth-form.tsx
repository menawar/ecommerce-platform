"use client";

// A Client Component because it needs interactivity: useActionState tracks the
// pending state and the error returned by the Server Action between submissions.
// The action itself still runs on the server — we just drive the UI around it.
import { useActionState } from "react";
import Link from "next/link";

import { InlineFormError } from "@/app/form-error";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
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
        <Input name="full_name" required placeholder="Full name" aria-label="Full name" autoComplete="name" />
      )}
      <Input
        name="email"
        type="email"
        required
        placeholder="Email"
        aria-label="Email"
        autoComplete="email"
      />
      <Input
        name="password"
        type="password"
        required
        minLength={8}
        placeholder="Password (min 8 chars)"
        aria-label="Password"
        autoComplete={isRegister ? "new-password" : "current-password"}
      />

      <InlineFormError message={state.error} />

      {!isRegister && (
        <div className="-mt-1 text-right">
          <Link href="/forgot-password" className="text-sm font-semibold text-accent">
            Forgot password?
          </Link>
        </div>
      )}

      <Button type="submit" size="lg" fullWidth loading={pending} className="mt-1">
        {isRegister ? "Create account" : "Sign in"}
      </Button>

      <p className="text-center text-sm text-fg-muted">
        {isRegister ? (
          <>
            Already have an account?{" "}
            <Link href="/login" className="font-semibold text-accent">
              Sign in
            </Link>
          </>
        ) : (
          <>
            New here?{" "}
            <Link href="/register" className="font-semibold text-accent">
              Create an account
            </Link>
          </>
        )}
      </p>
    </form>
  );
}
