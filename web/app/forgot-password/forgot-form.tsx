"use client";

import { useActionState } from "react";
import Link from "next/link";

import { InlineFormError } from "@/app/form-error";
import { Input } from "@/components/ui/input";
import { Button, buttonVariants } from "@/components/ui/button";
import { forgotPasswordAction } from "@/app/(auth)/actions";

export function ForgotForm() {
  const [state, formAction, pending] = useActionState(forgotPasswordAction, {});

  if (state.sent) {
    return (
      <div className="text-center">
        <p className="m-0 text-sm text-fg-muted">
          If an account exists for that email, we’ve sent a password-reset link. Check your inbox.
        </p>
        <Link href="/login" className={buttonVariants({ size: "lg" }) + " mt-6 w-full"}>
          Back to sign in
        </Link>
      </div>
    );
  }

  return (
    <form action={formAction} className="flex flex-col gap-3">
      <Input name="email" type="email" required placeholder="Email" aria-label="Email" autoComplete="email" />
      <InlineFormError message={state.error} />
      <Button type="submit" size="lg" fullWidth loading={pending} className="mt-1">
        Send reset link
      </Button>
      <p className="text-center text-sm text-fg-muted">
        Remembered it?{" "}
        <Link href="/login" className="font-semibold text-accent">
          Sign in
        </Link>
      </p>
    </form>
  );
}
