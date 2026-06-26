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
    <form action={formAction} style={{ marginTop: 16 }}>
      <input type="hidden" name="idempotency_key" value={idempotencyKey} />
      {state?.error && (
        <div
          style={{
            fontSize: 13,
            color: "var(--plt-error)",
            background: "var(--plt-error-bg)",
            padding: "10px 12px",
            borderRadius: "var(--plt-radius-sm)",
            marginBottom: 12,
          }}
        >
          {state.error}
        </div>
      )}
      <button disabled={pending} className="plt-btn-gold">
        {pending ? "Placing order…" : "Place order"}
      </button>
    </form>
  );
}
