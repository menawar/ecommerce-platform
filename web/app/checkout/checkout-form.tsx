"use client";

import { useActionState, useState } from "react";
import Link from "next/link";

import { InlineFormError } from "@/app/form-error";
import { cn } from "@/lib/cn";
import { formatPrice } from "@/lib/format";
import { Card } from "@/components/ui/card";
import { Button, buttonVariants } from "@/components/ui/button";
import type { Address } from "@/lib/addresses";
import type { ShippingMethod } from "@/lib/shipping";
import { placeOrderAction, type CheckoutState } from "./actions";

// A selectable radio "card" — highlighted when active.
const radioCard = (active: boolean) =>
  cn(
    "flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors",
    active ? "border-accent bg-accent-subtle" : "border-border-strong hover:bg-surface",
  );

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
      <Card className="text-center">
        <p className="m-0 mb-4 text-sm text-fg-muted">Add a delivery address to place your order.</p>
        <Link href="/account/addresses" className={buttonVariants({ size: "lg" })}>
          Add an address
        </Link>
      </Card>
    );
  }
  if (shippingMethods.length === 0) {
    return (
      <Card className="text-center">
        <p className="m-0 text-sm text-fg-muted">
          No shipping methods are available right now. Please check back shortly.
        </p>
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-5 lg:flex-row lg:items-start">
      {/* Pickers */}
      <div className="flex min-w-0 flex-1 flex-col gap-5">
        <Card>
          <h2 className="mb-3.5 text-base font-extrabold">Delivery address</h2>
          <div className="flex flex-col gap-2.5">
            {addresses.map((a) => (
              <label key={a.id} className={radioCard(a.id === addressID)}>
                <input
                  type="radio"
                  name="address_choice"
                  checked={a.id === addressID}
                  onChange={() => setAddressID(a.id)}
                  className="mt-1"
                />
                <div className="flex-1 text-sm">
                  <div className="font-bold">
                    {a.recipient}
                    {a.is_default && <span className="font-normal text-fg-muted"> · default</span>}
                  </div>
                  <div className="text-fg-muted">
                    {a.line1}
                    {a.line2 ? `, ${a.line2}` : ""}, {a.city}, {a.state}, {a.country}
                  </div>
                </div>
              </label>
            ))}
          </div>
          <Link
            href="/account/addresses"
            className="mt-3 inline-block text-sm font-semibold text-accent hover:underline"
          >
            Manage addresses
          </Link>
        </Card>

        <Card>
          <h2 className="mb-3.5 text-base font-extrabold">Delivery method</h2>
          <div className="flex flex-col gap-2.5">
            {shippingMethods.map((m) => (
              <label key={m.id} className={radioCard(m.id === shipID)}>
                <input
                  type="radio"
                  name="shipping_choice"
                  checked={m.id === shipID}
                  onChange={() => setShipID(m.id)}
                  className="mt-1"
                />
                <div className="flex-1">
                  <div className="text-sm font-bold">{m.name}</div>
                  {m.description && <div className="text-xs text-fg-muted">{m.description}</div>}
                </div>
                <div className="text-sm font-extrabold">
                  {m.price_cents === 0 ? "Free" : formatPrice(m.price_cents, currency)}
                </div>
              </label>
            ))}
          </div>
        </Card>
      </div>

      {/* Summary + submit */}
      <Card as="aside" className="w-full lg:w-[320px] lg:flex-none">
        <h2 className="mb-4 text-[17px] font-extrabold">Your order</h2>
        <Row label="Subtotal" value={formatPrice(subtotalCents, currency)} />
        <Row label="Delivery" value={shippingCents === 0 ? "Free" : formatPrice(shippingCents, currency)} />
        <div className="mt-1 flex justify-between border-t border-border-strong pt-3.5 text-lg font-extrabold">
          <span>Total</span>
          <span>{formatPrice(total, currency)}</span>
        </div>

        <form action={formAction} className="mt-4">
          <input type="hidden" name="idempotency_key" value={idempotencyKey} />
          <input type="hidden" name="address_id" value={addressID} />
          <input type="hidden" name="shipping_method_id" value={shipID} />
          <InlineFormError message={state?.error} style={{ marginBottom: 12 }} />
          <Button type="submit" variant="gold" size="lg" fullWidth loading={pending}>
            Place order
          </Button>
        </form>
      </Card>
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="mb-2.5 flex justify-between text-sm">
      <span className="text-fg-muted">{label}</span>
      <b>{value}</b>
    </div>
  );
}
