import "server-only";

import { gatewayFetch } from "./gateway";

// Mirrors the gateway's shipping-method DTO. price_cents is integer minor units.
export type ShippingMethod = {
  id: string;
  name: string;
  description: string;
  price_cents: number;
  sort_order: number;
  active: boolean;
};

export type ShippingMethodInput = {
  name: string;
  description: string;
  price_cents: number;
  sort_order: number;
  active: boolean;
};

// listShippingMethods returns active methods (the checkout view) by default. Pass
// includeInactive=true on the admin management page to get disabled ones too — the
// gateway only honors it for an admin caller.
export async function listShippingMethods(includeInactive = false): Promise<ShippingMethod[]> {
  const path = includeInactive ? "/shipping-methods?all=true" : "/shipping-methods";
  const res = await gatewayFetch<{ shipping_methods: ShippingMethod[] }>(path);
  return res.shipping_methods ?? [];
}

export async function createShippingMethod(input: ShippingMethodInput): Promise<ShippingMethod> {
  return gatewayFetch<ShippingMethod>("/shipping-methods", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateShippingMethod(id: string, input: ShippingMethodInput): Promise<ShippingMethod> {
  return gatewayFetch<ShippingMethod>(`/shipping-methods/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export async function deleteShippingMethod(id: string): Promise<void> {
  await gatewayFetch<void>(`/shipping-methods/${encodeURIComponent(id)}`, { method: "DELETE" });
}
