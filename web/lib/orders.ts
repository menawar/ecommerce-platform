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
// gateway passes it to the saga, which dedupes a retried submit to one order. In
// the async payment flow the saga returns PAYMENT_PENDING plus an authorization_url
// — the PSP hosted-checkout page the customer must visit to authorize payment.
export async function placeOrder(
  idempotencyKey: string,
): Promise<{ order_id: string; status: string; authorization_url: string }> {
  return gatewayFetch("/orders", {
    method: "POST",
    headers: { "Idempotency-Key": idempotencyKey },
  });
}

export async function getOrder(id: string): Promise<Order> {
  return gatewayFetch<Order>(`/orders/${encodeURIComponent(id)}`);
}

// listOrders returns the caller's orders, newest first. The gateway scopes the
// list to the authenticated user (user_id comes from the JWT, never the client),
// so there's no user filter to pass — the cookie is the identity.
export async function listOrders(): Promise<Order[]> {
  const { orders } = await gatewayFetch<{ orders: Order[] }>("/orders");
  return orders;
}
