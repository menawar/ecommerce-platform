import { describe, it, expect, vi } from "vitest";

// gateway.ts imports "server-only" which throws at import time in a non-server
// environment. Mock it to a no-op so we can test the module's exported logic.
vi.mock("server-only", () => ({}));

// next/headers is called at runtime by gatewayFetch (to read the cookie and the
// forwarded client IP). We stub both so the module loads without a Next.js runtime.
vi.mock("next/headers", () => ({
  cookies: vi.fn(() => Promise.resolve({ get: () => undefined })),
  headers: vi.fn(() => Promise.resolve({ get: () => null })),
}));

import { GatewayError, gatewayFetch } from "@/lib/gateway";

describe("GatewayError", () => {
  it("carries status code and message", () => {
    const err = new GatewayError(404, "not found");
    expect(err.status).toBe(404);
    expect(err.message).toBe("not found");
    expect(err.name).toBe("GatewayError");
  });

  it("carries an optional requestId from the gateway", () => {
    const err = new GatewayError(500, "boom", "abc-123");
    expect(err.requestId).toBe("abc-123");
  });

  it("is an instanceof Error (important for catch branches)", () => {
    const err = new GatewayError(422, "bad input");
    expect(err).toBeInstanceOf(Error);
    expect(err).toBeInstanceOf(GatewayError);
  });
});

describe("gatewayFetch", () => {
  it("throws GatewayError with status 503 when the gateway is unreachable", async () => {
    // globalThis.fetch rejecting simulates a network failure (DNS, connection refused).
    vi.stubGlobal(
      "fetch",
      vi.fn(() => Promise.reject(new TypeError("fetch failed"))),
    );

    await expect(gatewayFetch("/products")).rejects.toThrow(GatewayError);
    await expect(gatewayFetch("/products")).rejects.toMatchObject({
      status: 503,
    });

    vi.unstubAllGlobals();
  });

  it("throws GatewayError with the gateway's status on a non-2xx response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve({
          ok: false,
          status: 422,
          statusText: "Unprocessable Entity",
          headers: new Headers({ "X-Request-Id": "req-xyz" }),
          json: () => Promise.resolve({ error: "cart is empty" }),
        }),
      ),
    );

    try {
      await gatewayFetch("/orders");
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(GatewayError);
      const ge = err as GatewayError;
      expect(ge.status).toBe(422);
      expect(ge.message).toBe("cart is empty");
      expect(ge.requestId).toBe("req-xyz");
    }

    vi.unstubAllGlobals();
  });

  it("returns parsed JSON on a successful response", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ products: [], total: 0 }),
        }),
      ),
    );

    const result = await gatewayFetch<{ products: []; total: number }>("/products");
    expect(result).toEqual({ products: [], total: 0 });

    vi.unstubAllGlobals();
  });
});
