"use client";

import { useActionState, useEffect, useState } from "react";

import { InlineFormError } from "@/app/form-error";

import type { Address } from "@/lib/addresses";
import {
  saveAddressAction,
  deleteAddressAction,
  setDefaultAddressAction,
  type AddressFormState,
} from "./actions";

export function AddressManager({ addresses }: { addresses: Address[] }) {
  // `null` = form closed; "" = adding new; an id = editing that address.
  const [editing, setEditing] = useState<string | null>(null);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      {addresses.length === 0 && editing === null && (
        <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", margin: 0 }}>
          You have no saved addresses yet.
        </p>
      )}

      {addresses.map((a) =>
        editing === a.id ? (
          <AddressForm key={a.id} address={a} onClose={() => setEditing(null)} />
        ) : (
          <AddressCard key={a.id} address={a} onEdit={() => setEditing(a.id)} />
        ),
      )}

      {editing === "" ? (
        <AddressForm onClose={() => setEditing(null)} />
      ) : (
        editing === null && (
          <button className="plt-btn-outline" style={{ alignSelf: "flex-start" }} onClick={() => setEditing("")}>
            + Add address
          </button>
        )
      )}
    </div>
  );
}

function AddressCard({ address: a, onEdit }: { address: Address; onEdit: () => void }) {
  return (
    <div className="plt-card-lg" style={{ display: "flex", justifyContent: "space-between", gap: 16 }}>
      <div style={{ fontSize: 14, lineHeight: 1.5 }}>
        <div style={{ fontWeight: 700 }}>
          {a.recipient}
          {a.label && <span style={{ color: "var(--plt-text-secondary)", fontWeight: 400 }}> · {a.label}</span>}
          {a.is_default && (
            <span
              style={{
                marginLeft: 8,
                fontSize: 11,
                fontWeight: 700,
                background: "var(--plt-green-mid)",
                color: "#fff",
                borderRadius: 4,
                padding: "1px 6px",
              }}
            >
              Default
            </span>
          )}
        </div>
        <div style={{ color: "var(--plt-text-secondary)" }}>
          {a.line1}
          {a.line2 ? `, ${a.line2}` : ""}, {a.city}, {a.state}
          {a.postal_code ? ` ${a.postal_code}` : ""}, {a.country}
        </div>
        <div style={{ color: "var(--plt-text-secondary)" }}>{a.phone}</div>
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 6, alignItems: "flex-end" }}>
        <button className="plt-btn-outline" onClick={onEdit}>
          Edit
        </button>
        {!a.is_default && (
          <form action={setDefaultAddressAction}>
            <input type="hidden" name="id" value={a.id} />
            <button className="plt-btn-outline" style={{ whiteSpace: "nowrap" }}>
              Make default
            </button>
          </form>
        )}
        <form action={deleteAddressAction}>
          <input type="hidden" name="id" value={a.id} />
          <button className="plt-btn-outline" style={{ color: "var(--plt-error)" }}>
            Delete
          </button>
        </form>
      </div>
    </div>
  );
}

const fieldStyle = { display: "flex", flexDirection: "column" as const, gap: 4, fontSize: 13 };

function AddressForm({ address, onClose }: { address?: Address; onClose: () => void }) {
  const [state, formAction, pending] = useActionState<AddressFormState, FormData>(saveAddressAction, {});

  // Close the form once the save succeeds (the list re-renders via revalidatePath).
  useEffect(() => {
    if (state.ok) onClose();
  }, [state.ok, onClose]);

  return (
    <form action={formAction} className="plt-card-lg" style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      {address && <input type="hidden" name="id" value={address.id} />}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
        <label style={fieldStyle}>
          Recipient
          <input name="recipient" required defaultValue={address?.recipient} className="plt-input" />
        </label>
        <label style={fieldStyle}>
          Phone
          <input name="phone" required defaultValue={address?.phone} className="plt-input" />
        </label>
        <label style={fieldStyle}>
          Address line 1
          <input name="line1" required defaultValue={address?.line1} className="plt-input" />
        </label>
        <label style={fieldStyle}>
          Address line 2
          <input name="line2" defaultValue={address?.line2} className="plt-input" />
        </label>
        <label style={fieldStyle}>
          City
          <input name="city" required defaultValue={address?.city} className="plt-input" />
        </label>
        <label style={fieldStyle}>
          State
          <input name="state" required defaultValue={address?.state} className="plt-input" />
        </label>
        <label style={fieldStyle}>
          Postal code
          <input name="postal_code" defaultValue={address?.postal_code} className="plt-input" />
        </label>
        <label style={fieldStyle}>
          Label (optional)
          <input name="label" defaultValue={address?.label} placeholder="Home, Work…" className="plt-input" />
        </label>
      </div>
      {/* Only on create: the update path doesn't change the default flag (that's
          the "Make default" button on each card), so showing it on edit would be a
          live-but-dead control. */}
      {!address && (
        <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13 }}>
          <input type="checkbox" name="is_default" />
          Make this my default address
        </label>
      )}

      <InlineFormError message={state.error} />

      <div style={{ display: "flex", gap: 12 }}>
        <button disabled={pending} className="plt-btn-primary-lg" style={{ flex: 1 }}>
          {pending ? "Saving…" : address ? "Save changes" : "Add address"}
        </button>
        <button type="button" className="plt-btn-outline" onClick={onClose}>
          Cancel
        </button>
      </div>
    </form>
  );
}
