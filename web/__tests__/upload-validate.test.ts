import { describe, it, expect } from "vitest";

import {
  isAllowedImageType,
  imageObjectKey,
  MAX_IMAGE_BYTES,
} from "@/lib/upload-validate";

describe("isAllowedImageType", () => {
  it("accepts the supported image types", () => {
    for (const t of ["image/png", "image/jpeg", "image/webp", "image/gif"]) {
      expect(isAllowedImageType(t)).toBe(true);
    }
  });

  it("rejects everything else", () => {
    for (const t of ["application/pdf", "text/html", "image/svg+xml", "", "image/PNG"]) {
      expect(isAllowedImageType(t)).toBe(false);
    }
  });
});

describe("imageObjectKey", () => {
  it("derives the extension from the content-type, not the filename", () => {
    expect(imageObjectKey("image/jpeg", "abc-123")).toBe("products/abc-123.jpg");
    expect(imageObjectKey("image/png", "abc-123")).toBe("products/abc-123.png");
  });

  it("throws on an unsupported type", () => {
    expect(() => imageObjectKey("application/pdf", "abc-123")).toThrow();
  });
});

describe("MAX_IMAGE_BYTES", () => {
  it("is 5 MB", () => {
    expect(MAX_IMAGE_BYTES).toBe(5 * 1024 * 1024);
  });
});
