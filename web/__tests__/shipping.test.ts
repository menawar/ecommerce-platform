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
  listShippingMethods,
  createShippingMethod,
  updateShippingMethod,
  deleteShippingMethod,
  type ShippingMethodInput,
} from "@/lib/shipping";

const input: ShippingMethodInput = {
  name: "Express",
  description: "2 days",
  price_cents: 350000,
  sort_order: 2,
  active: true,
};

beforeEach(() => gatewayFetch.mockReset());

describe("lib/shipping", () => {
  it("listShippingMethods unwraps the array (active-only by default)", async () => {
    gatewayFetch.mockResolvedValue({ shipping_methods: [{ id: "s1" }] });
    expect(await listShippingMethods()).toEqual([{ id: "s1" }]);
    expect(gatewayFetch).toHaveBeenCalledWith("/shipping-methods");
  });

  it("listShippingMethods(true) requests the full admin list", async () => {
    gatewayFetch.mockResolvedValue({ shipping_methods: [] });
    await listShippingMethods(true);
    expect(gatewayFetch).toHaveBeenCalledWith("/shipping-methods?all=true");
  });

  it("listShippingMethods tolerates a missing array", async () => {
    gatewayFetch.mockResolvedValue({});
    expect(await listShippingMethods()).toEqual([]);
  });

  it("createShippingMethod POSTs the input", async () => {
    gatewayFetch.mockResolvedValue({ id: "s1" });
    await createShippingMethod(input);
    expect(gatewayFetch).toHaveBeenCalledWith("/shipping-methods", {
      method: "POST",
      body: JSON.stringify(input),
    });
  });

  it("updateShippingMethod PATCHes by id", async () => {
    gatewayFetch.mockResolvedValue({ id: "s9" });
    await updateShippingMethod("s9", input);
    expect(gatewayFetch).toHaveBeenCalledWith("/shipping-methods/s9", {
      method: "PATCH",
      body: JSON.stringify(input),
    });
  });

  it("deleteShippingMethod DELETEs by id", async () => {
    gatewayFetch.mockResolvedValue(undefined);
    await deleteShippingMethod("s9");
    expect(gatewayFetch).toHaveBeenCalledWith("/shipping-methods/s9", { method: "DELETE" });
  });
});
