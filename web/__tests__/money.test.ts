import { describe, it, expect } from "vitest";

import { parsePriceToCents } from "@/lib/money";

// The major→minor conversion is the one bit of real arithmetic in the admin
// create flow; a rounding or sign bug here mis-prices the whole catalog.
describe("parsePriceToCents", () => {
  it("converts a decimal major-unit price to integer cents", () => {
    expect(parsePriceToCents("19.99")).toBe(1999);
  });

  it("handles whole numbers and zero", () => {
    expect(parsePriceToCents("1500")).toBe(150000);
    expect(parsePriceToCents("0")).toBe(0);
  });

  it("rounds to the nearest cent (no floating-point drift)", () => {
    expect(parsePriceToCents("19.999")).toBe(2000);
    expect(parsePriceToCents("0.1")).toBe(10);
  });

  it("trims surrounding whitespace", () => {
    expect(parsePriceToCents("  9.50  ")).toBe(950);
  });

  it("rejects empty, non-numeric, and negative input", () => {
    expect(parsePriceToCents("")).toBeNull();
    expect(parsePriceToCents("   ")).toBeNull();
    expect(parsePriceToCents("abc")).toBeNull();
    expect(parsePriceToCents("-5")).toBeNull();
  });
});
