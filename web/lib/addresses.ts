import "server-only";

import { gatewayFetch } from "./gateway";

// Address mirrors the gateway's address DTO (no user_id — the caller is always the
// owner). created_at isn't surfaced; the list is already ordered default-first.
export type Address = {
  id: string;
  label: string;
  recipient: string;
  phone: string;
  line1: string;
  line2: string;
  city: string;
  state: string;
  postal_code: string;
  country: string;
  is_default: boolean;
};

// AddressInput is the create/update payload (mutable fields). is_default is only
// honored on create; switching the default later goes through setDefaultAddress.
export type AddressInput = {
  label: string;
  recipient: string;
  phone: string;
  line1: string;
  line2: string;
  city: string;
  state: string;
  postal_code: string;
  country: string;
  is_default?: boolean;
};

export async function listAddresses(): Promise<Address[]> {
  const res = await gatewayFetch<{ addresses: Address[] }>("/addresses");
  return res.addresses ?? [];
}

export async function createAddress(input: AddressInput): Promise<Address> {
  return gatewayFetch<Address>("/addresses", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateAddress(id: string, input: AddressInput): Promise<void> {
  await gatewayFetch<void>(`/addresses/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export async function deleteAddress(id: string): Promise<void> {
  await gatewayFetch<void>(`/addresses/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export async function setDefaultAddress(id: string): Promise<void> {
  await gatewayFetch<void>(`/addresses/${encodeURIComponent(id)}/default`, { method: "POST" });
}
