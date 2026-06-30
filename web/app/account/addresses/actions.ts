"use server";

import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";

import { GatewayError } from "@/lib/gateway";
import { createAddress, updateAddress, deleteAddress, setDefaultAddress, type AddressInput } from "@/lib/addresses";

export type AddressFormState = { error?: string; ok?: boolean };

function readInput(formData: FormData): AddressInput {
  const s = (k: string) => String(formData.get(k) ?? "").trim();
  return {
    label: s("label"),
    recipient: s("recipient"),
    phone: s("phone"),
    line1: s("line1"),
    line2: s("line2"),
    city: s("city"),
    state: s("state"),
    postal_code: s("postal_code"),
    country: s("country") || "NG",
    is_default: formData.get("is_default") === "on",
  };
}

// saveAddressAction handles both create (no id) and update (hidden id). Errors come
// back as state so the form can show them; success revalidates the list.
export async function saveAddressAction(_prev: AddressFormState, formData: FormData): Promise<AddressFormState> {
  const id = String(formData.get("id") ?? "").trim();
  const input = readInput(formData);
  try {
    if (id) {
      await updateAddress(id, input);
    } else {
      await createAddress(input);
    }
  } catch (err) {
    if (err instanceof GatewayError) {
      if (err.status === 401) redirect("/login");
      if (err.status === 400) return { error: err.message }; // e.g. "recipient is required"
      return { error: "Something went wrong — please try again." };
    }
    throw err;
  }
  revalidatePath("/account/addresses");
  return { ok: true };
}

export async function deleteAddressAction(formData: FormData) {
  const id = String(formData.get("id") ?? "");
  await deleteAddress(id);
  revalidatePath("/account/addresses");
}

export async function setDefaultAddressAction(formData: FormData) {
  const id = String(formData.get("id") ?? "");
  await setDefaultAddress(id);
  revalidatePath("/account/addresses");
}
