"use server";

import { redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { placeOrder } from "@/lib/orders";

export type CheckoutState = { error: string } | null;

export async function placeOrderAction(_prev: CheckoutState, formData: FormData): Promise<CheckoutState> {
  const key = String(formData.get("idempotency_key") ?? "");
  if (!key) {
    return { error: "Missing idempotency key — please refresh and try again." };
  }

  let res: { order_id: string; status: string };
  try {
    res = await placeOrder(key);
  } catch (err) {
    if (err instanceof GatewayError) {
      if (err.status === 401) redirect("/login");
      return { error: err.message }; // e.g. 422 "cart is empty"
    }
    throw err;
  }

  // Both CONFIRMED and CANCELLED are valid terminal outcomes (a decline is not an
  // error — it's a cancelled order). The order page renders whichever it is.
  redirect(`/orders/${res.order_id}`);
}
