import "server-only";

import { gatewayFetch } from "./gateway";

export type OrderItem = {
  product_id: string;
  name: string;
  price_cents: number;
  quantity: number;
};

export type ShippingAddress = {
  recipient: string;
  phone: string;
  line1: string;
  line2: string;
  city: string;
  state: string;
  postal_code: string;
  country: string;
};

export type Order = {
  id: string;
  status: string;
  total_cents: number; // subtotal + shipping
  shipping_cents: number;
  shipping_method_name: string;
  shipping_address?: ShippingAddress;
  currency: string;
  payment_id: string;
  created_at: number;
  items: OrderItem[];
};

// placeOrder forwards the client-generated key as the Idempotency-Key header (so a
// retried submit dedupes to one order) plus the chosen address + shipping method in
// the body. The async flow returns PAYMENT_PENDING + an authorization_url — the PSP
// hosted-checkout page the customer visits to authorize payment.
export async function placeOrder(
  idempotencyKey: string,
  addressID: string,
  shippingMethodID: string,
): Promise<{ order_id: string; status: string; authorization_url: string }> {
  return gatewayFetch("/orders", {
    method: "POST",
    headers: { "Idempotency-Key": idempotencyKey },
    body: JSON.stringify({ address_id: addressID, shipping_method_id: shippingMethodID }),
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
