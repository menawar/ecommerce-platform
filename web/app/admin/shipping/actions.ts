"use server";

import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";

import { GatewayError } from "@/lib/gateway";
import { parsePriceToCents } from "@/lib/money";
import { createShippingMethod, updateShippingMethod, deleteShippingMethod } from "@/lib/shipping";

export type ShippingFormState = { error?: string; ok?: boolean };

// saveShippingMethodAction handles create (no id) and update (hidden id). Price is
// entered in major units (e.g. 1500.00) and converted to integer cents here.
export async function saveShippingMethodAction(_prev: ShippingFormState, formData: FormData): Promise<ShippingFormState> {
  const id = String(formData.get("id") ?? "").trim();
  const cents = parsePriceToCents(String(formData.get("price") ?? ""));
  if (cents === null) {
    return { error: "Enter a valid price (e.g. 1500.00)." };
  }
  const input = {
    name: String(formData.get("name") ?? "").trim(),
    description: String(formData.get("description") ?? "").trim(),
    price_cents: cents,
    sort_order: Number(formData.get("sort_order")) || 0,
    active: formData.get("active") === "on",
  };
  try {
    if (id) {
      await updateShippingMethod(id, input);
    } else {
      await createShippingMethod(input);
    }
  } catch (err) {
    if (err instanceof GatewayError) {
      if (err.status === 401) redirect("/login");
      if (err.status === 400) return { error: err.message }; // e.g. "name is required"
      if (err.status === 403) return { error: "Admins only." };
      return { error: "Something went wrong — please try again." };
    }
    throw err;
  }
  revalidatePath("/admin/shipping");
  return { ok: true };
}

export async function deleteShippingMethodAction(formData: FormData) {
  const id = String(formData.get("id") ?? "").trim();
  if (!id) return;
  try {
    await deleteShippingMethod(id);
  } catch (err) {
    if (err instanceof GatewayError) {
      if (err.status === 401) redirect("/login");
      if (err.status === 404) {
        // Already deleted — nothing to do.
      } else {
        throw err;
      }
    } else {
      throw err;
    }
  }
  revalidatePath("/admin/shipping");
}
