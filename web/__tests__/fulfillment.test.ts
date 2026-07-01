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

import { listAllOrders, markShipped, markDelivered, refundOrder } from "@/lib/orders";

beforeEach(() => gatewayFetch.mockReset());

describe("lib/orders admin fulfillment", () => {
  it("listAllOrders GETs /admin/orders and unwraps", async () => {
    gatewayFetch.mockResolvedValue({ orders: [{ id: "o-1" }] });
    expect(await listAllOrders()).toEqual([{ id: "o-1" }]);
    expect(gatewayFetch).toHaveBeenCalledWith("/admin/orders");
  });

  it("listAllOrders tolerates a missing array", async () => {
    gatewayFetch.mockResolvedValue({});
    expect(await listAllOrders()).toEqual([]);
  });

  it("markShipped POSTs tracking to the ship sub-route", async () => {
    gatewayFetch.mockResolvedValue(undefined);
    await markShipped("o-1", "TRACK-9");
    expect(gatewayFetch).toHaveBeenCalledWith("/orders/o-1/ship", {
      method: "POST",
      body: JSON.stringify({ tracking_number: "TRACK-9" }),
    });
  });

  it("markDelivered POSTs to the deliver sub-route", async () => {
    gatewayFetch.mockResolvedValue(undefined);
    await markDelivered("o-1");
    expect(gatewayFetch).toHaveBeenCalledWith("/orders/o-1/deliver", { method: "POST" });
  });

  it("refundOrder POSTs to the refund sub-route", async () => {
    gatewayFetch.mockResolvedValue(undefined);
    await refundOrder("o-1");
    expect(gatewayFetch).toHaveBeenCalledWith("/orders/o-1/refund", { method: "POST" });
  });
});
