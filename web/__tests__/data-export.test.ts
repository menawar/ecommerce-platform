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

import { exportMyData } from "@/lib/data-export";

describe("exportMyData", () => {
  beforeEach(() => gatewayFetch.mockReset());

  it("GETs /me/export and returns the payload", async () => {
    const payload = { exported_at: "2026-07-01T00:00:00Z", profile: {}, addresses: [], orders: [] };
    gatewayFetch.mockResolvedValueOnce(payload);
    const out = await exportMyData();
    expect(gatewayFetch).toHaveBeenCalledWith("/me/export");
    expect(out).toBe(payload);
  });
});
