import { describe, it, expect, vi } from "vitest";

// Real GatewayError (server-only stubbed) so `instanceof` in the actions matches.
vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({
  cookies: vi.fn(() => Promise.resolve({ get: () => undefined })),
  headers: vi.fn(() => Promise.resolve({ get: () => null })),
}));
vi.mock("@/lib/session", () => ({
  requestPasswordReset: vi.fn(),
  resetPassword: vi.fn(),
  // actions.ts imports these too; stub so the module loads.
  login: vi.fn(),
  logout: vi.fn(),
  register: vi.fn(),
  resendVerification: vi.fn(),
  setSession: vi.fn(),
  verifyEmail: vi.fn(),
}));
const redirect = vi.fn((url: string) => {
  throw new Error(`REDIRECT:${url}`);
});
vi.mock("next/navigation", () => ({ redirect: (u: string) => redirect(u) }));

import { forgotPasswordAction, resetPasswordAction } from "@/app/(auth)/actions";
import { requestPasswordReset, resetPassword } from "@/lib/session";
import { GatewayError } from "@/lib/gateway";

const mockRequest = vi.mocked(requestPasswordReset);
const mockReset = vi.mocked(resetPassword);

function form(entries: Record<string, string>): FormData {
  const fd = new FormData();
  for (const [k, v] of Object.entries(entries)) fd.set(k, v);
  return fd;
}

describe("forgotPasswordAction", () => {
  it("returns sent on success (enumeration-safe confirmation)", async () => {
    mockRequest.mockImplementation(() => Promise.resolve());
    const state = await forgotPasswordAction({}, form({ email: "ada@example.com" }));
    expect(state.sent).toBe(true);
  });

  it("returns a generic error on a gateway failure", async () => {
    mockRequest.mockImplementation(() => {
      throw new GatewayError(503, "user service down");
    });
    const state = await forgotPasswordAction({}, form({ email: "ada@example.com" }));
    expect(state.error).toMatch(/went wrong/i);
  });
});

describe("resetPasswordAction", () => {
  it("redirects to /login on success", async () => {
    mockReset.mockImplementation(() => Promise.resolve());
    await expect(resetPasswordAction({}, form({ token: "t", password: "brand-new-pw" }))).rejects.toThrow(
      "REDIRECT:/login?reset=1",
    );
  });

  it("maps a 400 to an invalid-link message", async () => {
    mockReset.mockImplementation(() => {
      throw new GatewayError(400, "reset token is invalid or expired");
    });
    const state = await resetPasswordAction({}, form({ token: "nope", password: "brand-new-pw" }));
    expect(state.error).toMatch(/invalid or has expired/i);
  });
});
