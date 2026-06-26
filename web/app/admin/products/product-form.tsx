"use client";

import { useActionState } from "react";

import { createProductAction, type CreateProductState } from "./actions";

const labelStyle = { fontSize: 13, fontWeight: 700, display: "block", marginBottom: 4 };
const rowStyle = { marginBottom: 14 };

// The admin create-product form. useActionState wires the form to the server
// action and exposes pending state (to disable the button) and the returned
// error (rendered inline). On success the action redirects, so there's no
// success branch to render here.
export function ProductForm() {
  const [state, formAction, pending] = useActionState<CreateProductState, FormData>(
    createProductAction,
    null,
  );

  return (
    <form action={formAction} style={{ maxWidth: 460 }}>
      {state?.error && (
        <div
          style={{
            fontSize: 13,
            color: "var(--plt-error)",
            background: "var(--plt-error-bg)",
            padding: "10px 12px",
            borderRadius: "var(--plt-radius-sm)",
            marginBottom: 14,
          }}
        >
          {state.error}
        </div>
      )}

      <div style={rowStyle}>
        <label htmlFor="name" style={labelStyle}>Name</label>
        <input id="name" name="name" className="plt-input" required />
      </div>

      <div style={rowStyle}>
        <label htmlFor="sku" style={labelStyle}>SKU</label>
        <input id="sku" name="sku" className="plt-input" required />
      </div>

      <div style={{ display: "flex", gap: 12 }}>
        <div style={{ ...rowStyle, flex: 1 }}>
          <label htmlFor="price" style={labelStyle}>Price (₦)</label>
          <input
            id="price"
            name="price"
            type="number"
            min="0"
            step="0.01"
            className="plt-input"
            required
          />
        </div>
        <div style={{ ...rowStyle, flex: 1 }}>
          <label htmlFor="initial_quantity" style={labelStyle}>Initial stock</label>
          <input
            id="initial_quantity"
            name="initial_quantity"
            type="number"
            min="0"
            step="1"
            defaultValue={0}
            className="plt-input"
            required
          />
        </div>
      </div>

      <div style={rowStyle}>
        <label htmlFor="description" style={labelStyle}>Description</label>
        <textarea id="description" name="description" className="plt-input" rows={3} />
      </div>

      <div style={rowStyle}>
        <label htmlFor="image" style={labelStyle}>Image (optional)</label>
        <input
          id="image"
          name="image"
          type="file"
          accept="image/png,image/jpeg,image/webp,image/gif"
          className="plt-input"
        />
        <p style={{ fontSize: 11, color: "var(--plt-text-secondary)", marginTop: 4 }}>
          PNG, JPEG, WebP, or GIF · 5 MB max
        </p>
      </div>

      <button disabled={pending} className="plt-btn-gold" style={{ marginTop: 4 }}>
        {pending ? "Creating…" : "Create product"}
      </button>
    </form>
  );
}
