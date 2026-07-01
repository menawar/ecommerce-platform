"use server";

import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";

import { GatewayError } from "@/lib/gateway";
import { markShipped, markDelivered, refundOrder } from "@/lib/orders";

// shipOrderAction / deliverOrderAction advance fulfillment from the admin orders
// page. The gateway enforces the admin role; here we just map auth failures.
export async function shipOrderAction(formData: FormData) {
  const id = String(formData.get("id") ?? "").trim();
  const tracking = String(formData.get("tracking_number") ?? "").trim();
  if (!id) return;
  try {
    await markShipped(id, tracking);
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err; // 403 (not admin) / 422 (illegal transition) surface on the error boundary
  }
  revalidatePath("/admin/orders");
}

export async function deliverOrderAction(formData: FormData) {
  const id = String(formData.get("id") ?? "").trim();
  if (!id) return;
  try {
    await markDelivered(id);
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err;
  }
  revalidatePath("/admin/orders");
}

export async function refundOrderAction(formData: FormData) {
  const id = String(formData.get("id") ?? "").trim();
  if (!id) return;
  try {
    await refundOrder(id);
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err; // 403 / 422 (non-refundable) surface on the error boundary
  }
  revalidatePath("/admin/orders");
}
