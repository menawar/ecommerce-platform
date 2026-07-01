"use client";

import { useActionState } from "react";

import { InlineFormError } from "@/app/form-error";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { resetPasswordAction } from "@/app/(auth)/actions";

// The token rides in a hidden field and the password is only sent on submit (a
// POST), so nothing is consumed by a GET render or a link-scanner.
export function ResetForm({ token }: { token: string }) {
  const [state, formAction, pending] = useActionState(resetPasswordAction, {});

  return (
    <form action={formAction} className="flex flex-col gap-3">
      <input type="hidden" name="token" value={token} />
      <Input
        name="password"
        type="password"
        required
        minLength={8}
        placeholder="New password (min 8 chars)"
        aria-label="New password"
        autoComplete="new-password"
      />
      <InlineFormError message={state.error} />
      <Button type="submit" size="lg" fullWidth loading={pending} className="mt-1">
        Set new password
      </Button>
    </form>
  );
}
