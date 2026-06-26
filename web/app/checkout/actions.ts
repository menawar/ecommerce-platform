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

  let res: { order_id: string; status: string; authorization_url: string };
  try {
    res = await placeOrder(key);
  } catch (err) {
    if (err instanceof GatewayError) {
      if (err.status === 401) redirect("/login");
      return { error: err.message }; // e.g. 422 "cart is empty"
    }
    throw err;
  }

  // The async saga returns PAYMENT_PENDING + a hosted-checkout URL: send the
  // customer there to authorize payment. The PSP redirects them back to the order
  // page, which polls until the webhook settles the order to CONFIRMED/CANCELLED.
  if (res.authorization_url) {
    redirect(res.authorization_url);
  }

  // No authorization_url means the saga ended before payment (e.g. out of stock):
  // it's already terminal, so go straight to the order result page.
  redirect(`/orders/${res.order_id}`);
}
