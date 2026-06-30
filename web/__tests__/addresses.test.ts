import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({
  cookies: vi.fn(() => Promise.resolve({ get: () => undefined })),
  headers: vi.fn(() => Promise.resolve({ get: () => null })),
}));

const gatewayFetch = vi.fn();
vi.mock("@/lib/gateway", () => ({
  gatewayFetch: (...args: unknown[]) => gatewayFetch(...args),
}));

import {
  listAddresses,
  createAddress,
  updateAddress,
  deleteAddress,
  setDefaultAddress,
  type AddressInput,
} from "@/lib/addresses";

const input: AddressInput = {
  label: "Home",
  recipient: "Ada",
  phone: "0803",
  line1: "1 Rayfield",
  line2: "",
  city: "Jos",
  state: "Plateau",
  postal_code: "",
  country: "NG",
  is_default: true,
};

beforeEach(() => gatewayFetch.mockReset());

describe("lib/addresses", () => {
  it("listAddresses unwraps the addresses array", async () => {
    gatewayFetch.mockResolvedValue({ addresses: [{ id: "a-1" }] });
    const out = await listAddresses();
    expect(gatewayFetch).toHaveBeenCalledWith("/addresses");
    expect(out).toEqual([{ id: "a-1" }]);
  });

  it("listAddresses tolerates a missing array", async () => {
    gatewayFetch.mockResolvedValue({});
    expect(await listAddresses()).toEqual([]);
  });

  it("createAddress POSTs the input", async () => {
    gatewayFetch.mockResolvedValue({ id: "a-1" });
    await createAddress(input);
    expect(gatewayFetch).toHaveBeenCalledWith("/addresses", {
      method: "POST",
      body: JSON.stringify(input),
    });
  });

  it("updateAddress PATCHes by id", async () => {
    gatewayFetch.mockResolvedValue(undefined);
    await updateAddress("a-9", input);
    expect(gatewayFetch).toHaveBeenCalledWith("/addresses/a-9", {
      method: "PATCH",
      body: JSON.stringify(input),
    });
  });

  it("deleteAddress DELETEs by id", async () => {
    gatewayFetch.mockResolvedValue(undefined);
    await deleteAddress("a-9");
    expect(gatewayFetch).toHaveBeenCalledWith("/addresses/a-9", { method: "DELETE" });
  });

  it("setDefaultAddress POSTs to the default sub-route", async () => {
    gatewayFetch.mockResolvedValue(undefined);
    await setDefaultAddress("a-9");
    expect(gatewayFetch).toHaveBeenCalledWith("/addresses/a-9/default", { method: "POST" });
  });
});
