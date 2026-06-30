import { describe, it, expect, vi } from "vitest";

// Use the REAL GatewayError (so `instanceof` in the action matches what we throw)
// by stubbing only its server-only side-effects, exactly like gateway.test.ts.
vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({
  cookies: vi.fn(() => Promise.resolve({ get: () => undefined })),
  headers: vi.fn(() => Promise.resolve({ get: () => null })),
}));
vi.mock("@/lib/session", () => ({
  verifyEmail: vi.fn(),
  // actions.ts imports these too; stub so the module loads.
  login: vi.fn(),
  logout: vi.fn(),
  register: vi.fn(),
  resendVerification: vi.fn(),
  setSession: vi.fn(),
}));
vi.mock("next/navigation", () => ({ redirect: vi.fn() }));

import { verifyEmailAction } from "@/app/(auth)/actions";
import { verifyEmail } from "@/lib/session";
import { GatewayError } from "@/lib/gateway";

const mockVerify = vi.mocked(verifyEmail);

function form(token?: string): FormData {
  const fd = new FormData();
  if (token !== undefined) fd.set("token", token);
  return fd;
}

// NOTE: each test sets its own mockImplementation (and mockClear()s where call
// count matters) instead of a beforeEach(mockReset). Under vitest 4.1.9, resetting
// a mock between tests after a swallowed rejection mis-attributes that rejection as
// "unhandled" and fails the next test — the per-test mockImplementation pattern
// avoids stranding the promise.

describe("verifyEmailAction", () => {
  it("consumes the token and returns ok on success", async () => {
    mockVerify.mockClear();
    mockVerify.mockImplementation(() => Promise.resolve());
    const state = await verifyEmailAction({}, form("tok-abc"));
    expect(mockVerify).toHaveBeenCalledWith("tok-abc");
    expect(state.status).toBe("ok");
  });

  it("returns invalid (not throw) on a 400 from the gateway", async () => {
    mockVerify.mockImplementation(() => {
      throw new GatewayError(400, "verification token is invalid or expired");
    });
    const state = await verifyEmailAction({}, form("nope"));
    expect(state.status).toBe("invalid");
  });

  it("treats a missing token as invalid without calling the gateway", async () => {
    mockVerify.mockClear();
    mockVerify.mockImplementation(() => Promise.resolve());
    const state = await verifyEmailAction({}, form());
    expect(state.status).toBe("invalid");
    expect(mockVerify).not.toHaveBeenCalled();
  });

  it("rethrows a non-400 error to the boundary", async () => {
    mockVerify.mockImplementation(() => {
      throw new GatewayError(503, "user service down");
    });
    await expect(verifyEmailAction({}, form("tok"))).rejects.toThrow();
  });
});
