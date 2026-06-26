"use server";

import { redirect } from "next/navigation";

import { simulatePayment } from "@/lib/mock-psp";

// completeMockPaymentAction fires the simulated PSP webhook for this reference,
// then returns the customer to their order page (which polls until the saga
// settles). Dev-only — the real flow goes through the actual PSP.
export async function completeMockPaymentAction(formData: FormData): Promise<void> {
  const reference = String(formData.get("reference") ?? "");
  const orderId = String(formData.get("order_id") ?? "");
  if (reference) {
    await simulatePayment(reference);
  }
  redirect(orderId ? `/orders/${orderId}` : "/orders");
}
