import { describe, it, expect } from "vitest";
import { cn } from "@/lib/cn";

describe("cn", () => {
  it("joins truthy classes and drops falsy ones", () => {
    expect(cn("a", false && "b", null, undefined, "c")).toBe("a c");
  });

  it("later Tailwind utilities win over conflicting earlier ones", () => {
    expect(cn("px-2", "px-4")).toBe("px-4");
    expect(cn("text-fg", "text-fg-muted")).toBe("text-fg-muted");
  });

  it("supports conditional object syntax", () => {
    expect(cn("base", { active: true, hidden: false })).toBe("base active");
  });
});
