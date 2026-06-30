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
  const addressID = String(formData.get("address_id") ?? "");
  const shippingMethodID = String(formData.get("shipping_method_id") ?? "");
  if (!addressID || !shippingMethodID) {
    return { error: "Choose a delivery address and a shipping method." };
  }

  let res: { order_id: string; status: string; authorization_url: string };
  try {
    res = await placeOrder(key, addressID, shippingMethodID);
  } catch (err) {
    if (err instanceof GatewayError) {
      if (err.status === 401) redirect("/login");
      // requireVerified blocks checkout until the email is confirmed.
      if (err.status === 403) {
        return { error: "Please verify your email before checking out — check your inbox or resend the link from the banner above." };
      }
      return { error: err.message }; // e.g. 422 "cart is empty"
    }
    throw err;
  }

  // The async saga returns PAYMENT_PENDING + a hosted-checkout URL: send the
  // customer there to authorize payment. The PSP redirects them back to the order
  // page, which polls until the webhook settles the order to CONFIRMED/CANCELLED.
  if (res.authorization_url) {
    // A relative URL is the in-app mock-PSP simulator (dev only); thread the order
    // id so it can return the customer. A real PSP returns an absolute URL.
    if (res.authorization_url.startsWith("/")) {
      redirect(`${res.authorization_url}?order=${res.order_id}`);
    }
    redirect(res.authorization_url);
  }

  // No authorization_url means the saga ended before payment (e.g. out of stock):
  // it's already terminal, so go straight to the order result page.
  redirect(`/orders/${res.order_id}`);
}
