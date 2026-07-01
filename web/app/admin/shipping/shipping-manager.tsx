"use client";

import { useActionState, useEffect, useState } from "react";

import { InlineFormError } from "@/app/form-error";

import { formatPrice } from "@/lib/format";
import type { ShippingMethod } from "@/lib/shipping";
import { saveShippingMethodAction, deleteShippingMethodAction, type ShippingFormState } from "./actions";

export function ShippingManager({ methods }: { methods: ShippingMethod[] }) {
  // null = closed, "" = adding, id = editing that row.
  const [editing, setEditing] = useState<string | null>(null);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      {methods.length === 0 && editing === null && (
        <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", margin: 0 }}>No shipping methods yet.</p>
      )}

      {methods.map((m) =>
        editing === m.id ? (
          <ShippingForm key={m.id} method={m} onClose={() => setEditing(null)} />
        ) : (
          <MethodCard key={m.id} method={m} onEdit={() => setEditing(m.id)} />
        ),
      )}

      {editing === "" ? (
        <ShippingForm onClose={() => setEditing(null)} />
      ) : (
        editing === null && (
          <button className="plt-btn-outline" style={{ alignSelf: "flex-start" }} onClick={() => setEditing("")}>
            + Add shipping method
          </button>
        )
      )}
    </div>
  );
}

function MethodCard({ method: m, onEdit }: { method: ShippingMethod; onEdit: () => void }) {
  return (
    <div className="plt-card-lg" style={{ display: "flex", justifyContent: "space-between", gap: 16 }}>
      <div style={{ fontSize: 14, lineHeight: 1.5 }}>
        <div style={{ fontWeight: 700 }}>
          {m.name}
          {!m.active && (
            <span style={{ marginLeft: 8, fontSize: 11, fontWeight: 700, background: "#999", color: "#fff", borderRadius: 4, padding: "1px 6px" }}>
              Disabled
            </span>
          )}
        </div>
        {m.description && <div style={{ color: "var(--plt-text-secondary)" }}>{m.description}</div>}
        <div style={{ color: "var(--plt-text-secondary)" }}>
          {formatPrice(m.price_cents, "NGN")} · sort {m.sort_order}
        </div>
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 6, alignItems: "flex-end" }}>
        <button className="plt-btn-outline" onClick={onEdit}>
          Edit
        </button>
        <form action={deleteShippingMethodAction}>
          <input type="hidden" name="id" value={m.id} />
          <button className="plt-btn-outline" style={{ color: "var(--plt-error)" }}>
            Delete
          </button>
        </form>
      </div>
    </div>
  );
}

const fieldStyle = { display: "flex", flexDirection: "column" as const, gap: 4, fontSize: 13 };

function ShippingForm({ method, onClose }: { method?: ShippingMethod; onClose: () => void }) {
  const [state, formAction, pending] = useActionState<ShippingFormState, FormData>(saveShippingMethodAction, {});

  useEffect(() => {
    if (state.ok) onClose();
  }, [state.ok, onClose]);

  return (
    <form action={formAction} className="plt-card-lg" style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      {method && <input type="hidden" name="id" value={method.id} />}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
        <label style={fieldStyle}>
          Name
          <input name="name" required defaultValue={method?.name} className="plt-input" />
        </label>
        <label style={fieldStyle}>
          Price (NGN)
          <input
            name="price"
            required
            inputMode="decimal"
            defaultValue={method ? (method.price_cents / 100).toFixed(2) : ""}
            placeholder="1500.00"
            className="plt-input"
          />
        </label>
        <label style={fieldStyle}>
          Description
          <input name="description" defaultValue={method?.description} className="plt-input" />
        </label>
        <label style={fieldStyle}>
          Sort order
          <input name="sort_order" type="number" defaultValue={method?.sort_order ?? 0} className="plt-input" />
        </label>
      </div>
      <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13 }}>
        <input type="checkbox" name="active" defaultChecked={method ? method.active : true} />
        Active (shown at checkout)
      </label>

      <InlineFormError message={state.error} />

      <div style={{ display: "flex", gap: 12 }}>
        <button disabled={pending} className="plt-btn-primary-lg" style={{ flex: 1 }}>
          {pending ? "Saving…" : method ? "Save changes" : "Add method"}
        </button>
        <button type="button" className="plt-btn-outline" onClick={onClose}>
          Cancel
        </button>
      </div>
    </form>
  );
}
