"use client";

import { useActionState, useState } from "react";
import Link from "next/link";

import { formatPrice } from "@/lib/format";
import type { Address } from "@/lib/addresses";
import type { ShippingMethod } from "@/lib/shipping";
import { placeOrderAction, type CheckoutState } from "./actions";

export function CheckoutForm({
  subtotalCents,
  currency,
  addresses,
  shippingMethods,
}: {
  subtotalCents: number;
  currency: string;
  addresses: Address[];
  shippingMethods: ShippingMethod[];
}) {
  // One idempotency key per checkout attempt (stable across re-renders/double-clicks
  // so the saga dedupes a retried submit to a single order).
  const [idempotencyKey] = useState(() => crypto.randomUUID());
  const defaultAddr = addresses.find((a) => a.is_default) ?? addresses[0];
  const [addressID, setAddressID] = useState(defaultAddr?.id ?? "");
  const [shipID, setShipID] = useState(shippingMethods[0]?.id ?? "");
  const [state, formAction, pending] = useActionState<CheckoutState, FormData>(placeOrderAction, null);

  const selectedShip = shippingMethods.find((m) => m.id === shipID);
  const shippingCents = selectedShip?.price_cents ?? 0;
  const total = subtotalCents + shippingCents;

  if (addresses.length === 0) {
    return (
      <div className="plt-card-lg" style={{ textAlign: "center", padding: "32px 20px" }}>
        <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", margin: "0 0 16px" }}>
          Add a delivery address to place your order.
        </p>
        <Link href="/account/addresses" className="plt-btn-primary-lg" style={{ textDecoration: "none" }}>
          Add an address
        </Link>
      </div>
    );
  }
  if (shippingMethods.length === 0) {
    return (
      <div className="plt-card-lg" style={{ textAlign: "center", padding: "32px 20px" }}>
        <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", margin: 0 }}>
          No shipping methods are available right now. Please check back shortly.
        </p>
      </div>
    );
  }

  return (
    <div style={{ display: "flex", gap: 20, alignItems: "flex-start", flexWrap: "wrap" }}>
      {/* Left: pickers */}
      <div style={{ flex: 1, minWidth: 300, display: "flex", flexDirection: "column", gap: 18 }}>
        <div className="plt-card-lg">
          <div style={{ fontSize: 16, fontWeight: 800, marginBottom: 14 }}>Delivery address</div>
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {addresses.map((a) => (
              <label key={a.id} className={`plt-radio-card${a.id === addressID ? " active" : ""}`} style={{ cursor: "pointer" }}>
                <input
                  type="radio"
                  name="address_choice"
                  checked={a.id === addressID}
                  onChange={() => setAddressID(a.id)}
                  style={{ marginRight: 4 }}
                />
                <div style={{ flex: 1, fontSize: 13 }}>
                  <div style={{ fontWeight: 700 }}>
                    {a.recipient}
                    {a.is_default && <span style={{ color: "var(--plt-text-secondary)", fontWeight: 400 }}> · default</span>}
                  </div>
                  <div style={{ color: "var(--plt-text-secondary)" }}>
                    {a.line1}
                    {a.line2 ? `, ${a.line2}` : ""}, {a.city}, {a.state}, {a.country}
                  </div>
                </div>
              </label>
            ))}
          </div>
          <Link href="/account/addresses" style={{ fontSize: 13, color: "var(--plt-terracotta)", fontWeight: 600, display: "inline-block", marginTop: 12 }}>
            Manage addresses
          </Link>
        </div>

        <div className="plt-card-lg">
          <div style={{ fontSize: 16, fontWeight: 800, marginBottom: 14 }}>Delivery method</div>
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {shippingMethods.map((m) => (
              <label key={m.id} className={`plt-radio-card${m.id === shipID ? " active" : ""}`} style={{ cursor: "pointer" }}>
                <input
                  type="radio"
                  name="shipping_choice"
                  checked={m.id === shipID}
                  onChange={() => setShipID(m.id)}
                  style={{ marginRight: 4 }}
                />
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 14, fontWeight: 700 }}>{m.name}</div>
                  {m.description && (
                    <div style={{ fontSize: 12, color: "var(--plt-text-secondary)" }}>{m.description}</div>
                  )}
                </div>
                <div style={{ fontSize: 14, fontWeight: 800 }}>
                  {m.price_cents === 0 ? "Free" : formatPrice(m.price_cents, currency)}
                </div>
              </label>
            ))}
          </div>
        </div>
      </div>

      {/* Right: summary + submit */}
      <div className="plt-card-lg" style={{ width: 320, flex: "0 0 320px" }}>
        <div style={{ fontSize: 17, fontWeight: 800, marginBottom: 16 }}>Your order</div>
        <Row label="Subtotal" value={formatPrice(subtotalCents, currency)} />
        <Row label="Delivery" value={shippingCents === 0 ? "Free" : formatPrice(shippingCents, currency)} />
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            fontSize: 18,
            fontWeight: 800,
            borderTop: "1px solid var(--plt-border-heavy)",
            paddingTop: 14,
            marginTop: 4,
          }}
        >
          <span>Total</span>
          <span>{formatPrice(total, currency)}</span>
        </div>

        <form action={formAction} style={{ marginTop: 16 }}>
          <input type="hidden" name="idempotency_key" value={idempotencyKey} />
          <input type="hidden" name="address_id" value={addressID} />
          <input type="hidden" name="shipping_method_id" value={shipID} />
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
          <button disabled={pending} className="plt-btn-gold" style={{ width: "100%" }}>
            {pending ? "Placing order…" : "Place order"}
          </button>
        </form>
      </div>
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", fontSize: 14, marginBottom: 10 }}>
      <span style={{ color: "var(--plt-text-secondary)" }}>{label}</span>
      <b>{value}</b>
    </div>
  );
}
