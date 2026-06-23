"use client";

import { useActionState, useState } from "react";

import { placeOrderAction, type CheckoutState } from "./actions";

export function CheckoutButton() {
  // ONE idempotency key per checkout ATTEMPT. The useState initializer runs once
  // (on mount), so re-renders and double-clicks reuse the SAME key — and the saga
  // dedupes a retried submit to a single order/charge. A fresh page load (a new
  // deliberate attempt) generates a new key. This is why the key is client-side:
  // it must be stable across the retries the network/user might trigger.
  const [idempotencyKey] = useState(() => crypto.randomUUID());

  // useActionState disables the button while the action runs (pending), so a
  // double-click can't even fire twice — defense in depth alongside the key.
  const [state, formAction, pending] = useActionState<CheckoutState, FormData>(placeOrderAction, null);

  return (
    <form action={formAction} className="mt-6">
      <input type="hidden" name="idempotency_key" value={idempotencyKey} />
      {state?.error && <p className="mb-3 text-sm text-red-600">{state.error}</p>}
      <button
        disabled={pending}
        className="rounded-md bg-foreground px-5 py-2.5 font-medium text-background disabled:opacity-60"
      >
        {pending ? "Placing order…" : "Place order"}
      </button>
    </form>
  );
}
