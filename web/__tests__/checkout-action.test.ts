import { describe, it, expect, vi, beforeEach } from "vitest";

// The mock factory is hoisted above imports, so the GatewayError stand-in must be
// DEFINED inside it (no top-level references). We re-import it below to construct
// errors the action's `instanceof GatewayError` branch recognizes.
vi.mock("@/lib/gateway", () => ({
  GatewayError: class GatewayError extends Error {
    constructor(
      public status: number,
      message: string,
    ) {
      super(message);
      this.name = "GatewayError";
    }
  },
}));
vi.mock("@/lib/orders", () => ({ placeOrder: vi.fn() }));
vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`REDIRECT:${url}`);
  }),
}));

import { placeOrderAction } from "@/app/checkout/actions";
import { placeOrder } from "@/lib/orders";
import { GatewayError } from "@/lib/gateway";

const mockPlaceOrder = vi.mocked(placeOrder);

function form(key = "k-1"): FormData {
  const fd = new FormData();
  fd.set("idempotency_key", key);
  return fd;
}

beforeEach(() => {
  mockPlaceOrder.mockReset();
});

describe("placeOrderAction verification gate", () => {
  it("maps a 403 to a verify-your-email message instead of the raw gateway text", async () => {
    mockPlaceOrder.mockRejectedValue(new GatewayError(403, "email verification required"));
    const state = await placeOrderAction(null, form());
    expect(state?.error).toMatch(/verify your email/i);
  });

  it("passes through a 422 business error verbatim", async () => {
    mockPlaceOrder.mockRejectedValue(new GatewayError(422, "cart is empty"));
    const state = await placeOrderAction(null, form());
    expect(state?.error).toBe("cart is empty");
  });

  it("redirects to /login on a 401", async () => {
    mockPlaceOrder.mockRejectedValue(new GatewayError(401, "unauthorized"));
    await expect(placeOrderAction(null, form())).rejects.toThrow("REDIRECT:/login");
  });
});
