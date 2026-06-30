import { describe, it, expect, vi, beforeEach } from "vitest";

// session.ts imports "server-only" and next/headers; stub both so it loads outside
// a Next runtime. The gateway module is mocked so we can assert the exact call.
vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({
  cookies: vi.fn(() => Promise.resolve({ get: () => undefined, set: vi.fn(), delete: vi.fn() })),
}));

const gatewayFetch = vi.fn();
vi.mock("@/lib/gateway", () => ({
  gatewayFetch: (...args: unknown[]) => gatewayFetch(...args),
  SESSION_COOKIE: "session",
  GatewayError: class GatewayError extends Error {},
}));

import { verifyEmail, resendVerification } from "@/lib/session";

beforeEach(() => {
  gatewayFetch.mockReset();
  gatewayFetch.mockResolvedValue(undefined);
});

describe("verifyEmail", () => {
  it("POSTs the token to /auth/verify-email", async () => {
    await verifyEmail("tok-abc");
    expect(gatewayFetch).toHaveBeenCalledWith("/auth/verify-email", {
      method: "POST",
      body: JSON.stringify({ token: "tok-abc" }),
    });
  });
});

describe("resendVerification", () => {
  it("POSTs to /auth/resend-verification with no body (user from cookie)", async () => {
    await resendVerification();
    expect(gatewayFetch).toHaveBeenCalledWith("/auth/resend-verification", { method: "POST" });
  });
});
