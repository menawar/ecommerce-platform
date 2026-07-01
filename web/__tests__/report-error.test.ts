import { describe, it, expect, vi, afterEach } from "vitest";

import { reportError } from "@/lib/report-error";

describe("reportError", () => {
  afterEach(() => vi.restoreAllMocks());

  it("logs structured error context without throwing", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    reportError(new Error("boom"), { path: "/checkout" });
    expect(spy).toHaveBeenCalledOnce();
    const logged = spy.mock.calls[0].join(" ");
    expect(logged).toContain("boom");
    expect(logged).toContain("/checkout");
  });

  it("handles non-Error values", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => reportError("plain failure")).not.toThrow();
    expect(spy.mock.calls[0].join(" ")).toContain("plain failure");
  });
});
