"use client";

import { deleteProductAction } from "./actions";

// A per-row delete control. Client component only for the confirm() guard on a
// destructive action; the actual work is the server action. On confirm, the form
// posts to deleteProductAction (which archives the product and revalidates).
export function DeleteProductButton({ id, name }: { id: string; name: string }) {
  return (
    <form
      action={deleteProductAction}
      onSubmit={(e) => {
        if (!confirm(`Delete "${name}"? It will be removed from the store.`)) {
          e.preventDefault();
        }
      }}
      style={{ display: "inline" }}
    >
      <input type="hidden" name="id" value={id} />
      <button
        type="submit"
        style={{
          background: "none",
          border: 0,
          color: "var(--plt-error)",
          cursor: "pointer",
          fontWeight: 600,
          fontSize: 13,
          padding: 0,
        }}
      >
        Delete
      </button>
    </form>
  );
}
