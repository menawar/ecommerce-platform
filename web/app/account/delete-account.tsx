"use client";

import { useState } from "react";
import { deleteAccountAction } from "./actions";

// DeleteAccountButton is a two-step, confirm-gated control for an irreversible,
// outward-facing action — a single misclick shouldn't erase an account.
export function DeleteAccountButton() {
  const [confirming, setConfirming] = useState(false);

  if (!confirming) {
    return (
      <button
        type="button"
        onClick={() => setConfirming(true)}
        className="plt-btn-outline"
        style={{ color: "var(--plt-error)", borderColor: "var(--plt-error)" }}
      >
        Delete my account
      </button>
    );
  }

  return (
    <div style={{ border: "1px solid var(--plt-error)", borderRadius: 10, padding: 16 }}>
      <p style={{ margin: "0 0 12px", fontSize: 14 }}>
        This <b>permanently</b> deletes your account and anonymises your personal data. Your past
        orders are kept in anonymised form for accounting. This cannot be undone.
      </p>
      <div style={{ display: "flex", gap: 10 }}>
        <form action={deleteAccountAction}>
          <button
            type="submit"
            className="plt-btn-outline"
            style={{ color: "var(--plt-error)", borderColor: "var(--plt-error)" }}
          >
            Yes, delete permanently
          </button>
        </form>
        <button type="button" onClick={() => setConfirming(false)} className="plt-btn-outline">
          Cancel
        </button>
      </div>
    </div>
  );
}
