import { describe, it, expect } from "vitest";

import { formatPrice, formatDate } from "@/lib/format";

// Pure-logic unit tests — no React, no Next.js, no mocking needed. These prove
// the two formatting helpers that every money-displaying component depends on.

describe("formatPrice", () => {
  it("formats kobo as NGN with two decimals", () => {
    // 150000 kobo = ₦1,500.00
    const result = formatPrice(150000, "NGN");
    // Intl.NumberFormat may use a narrow non-breaking space as a group separator
    // depending on the ICU version, so we normalize whitespace for a stable assertion.
    const normalized = result.replace(/\s/g, " ");
    expect(normalized).toContain("1,500.00");
  });

  it("handles zero", () => {
    expect(formatPrice(0, "NGN")).toContain("0.00");
  });

  it("defaults to NGN for empty currency", () => {
    const result = formatPrice(99, "");
    expect(result).toContain("0.99");
  });

  it("formats USD when given a different currency", () => {
    const result = formatPrice(1234, "USD");
    expect(result).toContain("12.34");
  });
});

describe("formatDate", () => {
  it("converts epoch seconds (not ms) to a human-readable date", () => {
    // 2025-01-15T12:00:00Z = 1736942400 seconds
    const result = formatDate(1736942400);
    expect(result).toContain("2025");
    // Just verify it produced a real date, not "Jan 1, 1970" (the classic bug
    // when you forget to × 1000).
    expect(result).not.toContain("1970");
  });
});
