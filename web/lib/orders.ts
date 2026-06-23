import "server-only";

import { gatewayFetch } from "./gateway";

export type OrderItem = {
  product_id: string;
  name: string;
  price_cents: number;
  quantity: number;
};

export type Order = {
  id: string;
  status: string;
  total_cents: number;
  currency: string;
  payment_id: string;
  created_at: number;
  items: OrderItem[];
};

// placeOrder forwards the client-generated key as the Idempotency-Key header. The
// gateway passes it to the saga, which dedupes a retried submit to one order.
export async function placeOrder(idempotencyKey: string): Promise<{ order_id: string; status: string }> {
  return gatewayFetch("/orders", {
    method: "POST",
    headers: { "Idempotency-Key": idempotencyKey },
  });
}

export async function getOrder(id: string): Promise<Order> {
  return gatewayFetch<Order>(`/orders/${encodeURIComponent(id)}`);
}
