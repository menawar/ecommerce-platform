import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({
  cookies: vi.fn(() => Promise.resolve({ get: () => undefined })),
  headers: vi.fn(() => Promise.resolve({ get: () => null })),
}));

const listProducts = vi.fn();
vi.mock("@/lib/gateway", () => ({
  listProducts: (...args: unknown[]) => listProducts(...args),
  GatewayError: class extends Error {},
}));

import robots from "@/app/robots";
import sitemap from "@/app/sitemap";

describe("robots", () => {
  it("allows crawling but disallows private areas and links the sitemap", () => {
    const r = robots();
    const rule = Array.isArray(r.rules) ? r.rules[0] : r.rules;
    expect(rule.allow).toBe("/");
    expect(rule.disallow).toContain("/account");
    expect(rule.disallow).toContain("/admin");
    expect(r.sitemap).toMatch(/\/sitemap\.xml$/);
  });
});

describe("sitemap", () => {
  beforeEach(() => listProducts.mockReset());

  it("includes static routes plus a URL per product", async () => {
    listProducts.mockResolvedValueOnce({ products: [{ id: "p-1", created_at: 1700000000 }] });
    const entries = await sitemap();
    const urls = entries.map((e) => e.url);
    expect(urls.some((u) => u.endsWith("/products"))).toBe(true);
    expect(urls.some((u) => u.endsWith("/products/p-1"))).toBe(true);
  });

  it("still returns static routes if the gateway is unreachable", async () => {
    listProducts.mockRejectedValueOnce(new Error("gateway down"));
    const entries = await sitemap();
    expect(entries.length).toBeGreaterThan(0);
    expect(entries.some((e) => e.url.endsWith("/products"))).toBe(true);
  });
});
