import { describe, it, expect, afterEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";

import { ProductCard } from "@/components/product-card";
import type { Product } from "@/lib/types";

const base: Product = {
  id: "p-1",
  sku: "SKU-1",
  name: "Jos Plateau Potatoes",
  description: "",
  price_cents: 250000,
  currency: "NGN",
  category_id: "c-1",
  available: 12,
  created_at: 0,
  image_url: "",
};

describe("ProductCard", () => {
  afterEach(cleanup);

  it("links to the product and shows name + price", () => {
    render(<ProductCard product={base} />);
    expect(screen.getByRole("link")).toHaveAttribute("href", "/products/p-1");
    expect(screen.getByText("Jos Plateau Potatoes")).toBeInTheDocument();
    expect(screen.getByText("₦2,500.00")).toBeInTheDocument();
  });

  it("shows an In stock badge + delivery line when available", () => {
    render(<ProductCard product={base} />);
    expect(screen.getByText("In stock")).toBeInTheDocument();
    expect(screen.getByText("Delivered this week")).toBeInTheDocument();
  });

  it("shows Sold out when unavailable and renders the SKU placeholder without an image", () => {
    render(<ProductCard product={{ ...base, available: 0 }} />);
    expect(screen.getByText("Sold out")).toBeInTheDocument();
    expect(screen.getByText("SKU-1")).toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("renders an <img> with alt when an image_url is present (no fabricated ratings)", () => {
    render(<ProductCard product={{ ...base, image_url: "https://x/y.jpg" }} />);
    expect(screen.getByRole("img")).toHaveAttribute("alt", "Jos Plateau Potatoes");
    expect(screen.queryByText(/★/)).not.toBeInTheDocument();
  });
});
