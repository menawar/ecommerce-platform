"use client";

import { useActionState } from "react";
import { InlineFormError } from "@/app/form-error";

import type { Product } from "@/lib/types";
import { updateProductAction, type EditProductState } from "./actions";

const labelStyle = { fontSize: 13, fontWeight: 700, display: "block", marginBottom: 4 };
const rowStyle = { marginBottom: 14 };

// The admin edit form, pre-filled from the current product. SKU is shown read-only
// (immutable). Stock is optional — blank leaves it unchanged — with the current
// available count shown as a reference.
export function EditForm({ product }: { product: Product }) {
  const [state, formAction, pending] = useActionState<EditProductState, FormData>(
    updateProductAction,
    null,
  );

  return (
    <form action={formAction} style={{ maxWidth: 460 }}>
      {/* Authoritative id + fields not edited here, carried through so the
          full-replace update preserves them. */}
      <input type="hidden" name="id" value={product.id} />
      <input type="hidden" name="current_image_url" value={product.image_url} />
      <input type="hidden" name="category_id" value={product.category_id} />

      <InlineFormError message={state?.error} style={{ marginBottom: 14 }} />

      <div style={rowStyle}>
        <label htmlFor="sku" style={labelStyle}>SKU (cannot be changed)</label>
        <input id="sku" value={product.sku} disabled className="plt-input" />
      </div>

      <div style={rowStyle}>
        <label htmlFor="name" style={labelStyle}>Name</label>
        <input id="name" name="name" defaultValue={product.name} className="plt-input" required />
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
            defaultValue={(product.price_cents / 100).toFixed(2)}
            className="plt-input"
            required
          />
        </div>
        <div style={{ ...rowStyle, flex: 1 }}>
          <label htmlFor="quantity" style={labelStyle}>Stock</label>
          <input
            id="quantity"
            name="quantity"
            type="number"
            min="0"
            step="1"
            placeholder={`${product.available} (blank = keep)`}
            className="plt-input"
          />
        </div>
      </div>

      <div style={rowStyle}>
        <label htmlFor="description" style={labelStyle}>Description</label>
        <textarea id="description" name="description" defaultValue={product.description} className="plt-input" rows={3} />
      </div>

      <div style={rowStyle}>
        <label htmlFor="image" style={labelStyle}>Replace image (optional)</label>
        {product.image_url && (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={product.image_url}
            alt={product.name}
            style={{ display: "block", width: 96, height: 96, objectFit: "cover", borderRadius: 6, marginBottom: 8 }}
          />
        )}
        <input
          id="image"
          name="image"
          type="file"
          accept="image/png,image/jpeg,image/webp,image/gif"
          className="plt-input"
        />
        <p style={{ fontSize: 11, color: "var(--plt-text-secondary)", marginTop: 4 }}>
          PNG, JPEG, WebP, or GIF · 5 MB max · leave empty to keep the current image
        </p>
      </div>

      <button disabled={pending} className="plt-btn-gold" style={{ marginTop: 4 }}>
        {pending ? "Saving…" : "Save changes"}
      </button>
    </form>
  );
}
