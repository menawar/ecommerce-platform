"use client";

import { useActionState, useEffect, useState } from "react";

import { InlineFormError } from "@/app/form-error";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
    <div className="flex flex-col gap-4">
      {addresses.length === 0 && editing === null && (
        <p className="m-0 text-sm text-fg-muted">You have no saved addresses yet.</p>
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
          <Button variant="outline" className="self-start" onClick={() => setEditing("")}>
            + Add address
          </Button>
        )
      )}
    </div>
  );
}

function AddressCard({ address: a, onEdit }: { address: Address; onEdit: () => void }) {
  return (
    <Card className="flex justify-between gap-4">
      <div className="min-w-0 text-sm leading-relaxed">
        <div className="font-bold">
          {a.recipient}
          {a.label && <span className="font-normal text-fg-muted"> · {a.label}</span>}
          {a.is_default && (
            <Badge variant="success" className="ml-2">
              Default
            </Badge>
          )}
        </div>
        <div className="text-fg-muted">
          {a.line1}
          {a.line2 ? `, ${a.line2}` : ""}, {a.city}, {a.state}
          {a.postal_code ? ` ${a.postal_code}` : ""}, {a.country}
        </div>
        <div className="text-fg-muted">{a.phone}</div>
      </div>
      <div className="flex flex-col items-end gap-1.5">
        <Button variant="outline" size="sm" onClick={onEdit}>
          Edit
        </Button>
        {!a.is_default && (
          <form action={setDefaultAddressAction}>
            <input type="hidden" name="id" value={a.id} />
            <Button type="submit" variant="outline" size="sm" className="whitespace-nowrap">
              Make default
            </Button>
          </form>
        )}
        <form action={deleteAddressAction}>
          <input type="hidden" name="id" value={a.id} />
          <Button type="submit" variant="outline" size="sm" className="text-danger">
            Delete
          </Button>
        </form>
      </div>
    </Card>
  );
}

function LabeledInput({
  label,
  ...props
}: { label: string } & React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="font-semibold">{label}</span>
      <Input {...props} />
    </label>
  );
}

function AddressForm({ address, onClose }: { address?: Address; onClose: () => void }) {
  const [state, formAction, pending] = useActionState<AddressFormState, FormData>(saveAddressAction, {});

  // Close the form once the save succeeds (the list re-renders via revalidatePath).
  useEffect(() => {
    if (state.ok) onClose();
  }, [state.ok, onClose]);

  return (
    <form action={formAction} className="flex flex-col gap-3 rounded-xl border border-border bg-card p-5 shadow-card">
      {address && <input type="hidden" name="id" value={address.id} />}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <LabeledInput label="Recipient" name="recipient" required defaultValue={address?.recipient} />
        <LabeledInput label="Phone" name="phone" required defaultValue={address?.phone} />
        <LabeledInput label="Address line 1" name="line1" required defaultValue={address?.line1} />
        <LabeledInput label="Address line 2" name="line2" defaultValue={address?.line2} />
        <LabeledInput label="City" name="city" required defaultValue={address?.city} />
        <LabeledInput label="State" name="state" required defaultValue={address?.state} />
        <LabeledInput label="Postal code" name="postal_code" defaultValue={address?.postal_code} />
        <LabeledInput label="Label (optional)" name="label" defaultValue={address?.label} placeholder="Home, Work…" />
      </div>
      {/* Only on create: the update path doesn't change the default flag (that's the
          "Make default" button on each card), so showing it on edit would be a
          live-but-dead control. */}
      {!address && (
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" name="is_default" />
          Make this my default address
        </label>
      )}

      <InlineFormError message={state.error} />

      <div className="flex gap-3">
        <Button type="submit" size="lg" loading={pending} className="flex-1">
          {address ? "Save changes" : "Add address"}
        </Button>
        <Button type="button" variant="outline" size="lg" onClick={onClose}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
