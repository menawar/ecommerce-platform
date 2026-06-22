"use server";

import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";

import { GatewayError } from "@/lib/gateway";
import { addToCart, updateCartItem, removeCartItem } from "@/lib/cart";

// These are the "mutate then revalidate" Server Actions. After changing the cart
// in Redis (via the gateway), revalidatePath("/cart") tells Next to drop any
// cached render of the cart route so the next view reflects the change. The forms
// that call these are plain <form>s — no client JS, full progressive enhancement.

export async function addToCartAction(formData: FormData) {
  const productID = String(formData.get("product_id") ?? "");
  const quantity = Number(formData.get("quantity")) || 1;

  try {
    await addToCart(productID, quantity);
  } catch (err) {
    // Adding requires a session; an unauthenticated/expired user gets bounced.
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err;
  }
  revalidatePath("/cart");
  redirect("/cart"); // show the cart so the add is visibly confirmed
}

export async function updateCartItemAction(formData: FormData) {
  const productID = String(formData.get("product_id") ?? "");
  const quantity = Number(formData.get("quantity")) || 0;

  await updateCartItem(productID, quantity);
  revalidatePath("/cart");
}

export async function removeCartItemAction(formData: FormData) {
  const productID = String(formData.get("product_id") ?? "");

  await removeCartItem(productID);
  revalidatePath("/cart");
}
