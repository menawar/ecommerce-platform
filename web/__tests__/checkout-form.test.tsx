import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

// CheckoutForm imports ./actions (a "use server" file) which can't run in jsdom;
// mock it. The POINT here is the form wiring (idempotency key + chosen ids), not
// the Server Action itself.
vi.mock("@/app/checkout/actions", () => ({
  placeOrderAction: vi.fn(),
}));

// useActionState isn't available in plain Vitest+jsdom; stub it.
vi.mock("react", async () => {
  const actual = await vi.importActual<typeof import("react")>("react");
  return {
    ...actual,
    useActionState: (_action: unknown, initialState: unknown) => [initialState, vi.fn(), false],
  };
});

import { CheckoutForm } from "@/app/checkout/checkout-form";
import type { Address } from "@/lib/addresses";
import type { ShippingMethod } from "@/lib/shipping";

const addr: Address = {
  id: "a-1", label: "", recipient: "Ada", phone: "0803", line1: "1 Rayfield", line2: "",
  city: "Jos", state: "Plateau", postal_code: "", country: "NG", is_default: true,
};
const ship: ShippingMethod = {
  id: "s-1", name: "Standard", description: "2-3 days", price_cents: 150000, sort_order: 1, active: true,
};

beforeEach(() => vi.clearAllMocks());

describe("CheckoutForm", () => {
  it("renders Place order with hidden idempotency key + chosen ids", () => {
    const { container } = render(
      <CheckoutForm subtotalCents={250000} currency="NGN" addresses={[addr]} shippingMethods={[ship]} />,
    );
    expect(screen.getByRole("button", { name: /place order/i })).toBeInTheDocument();

    const key = container.querySelector('input[name="idempotency_key"]') as HTMLInputElement;
    expect(key?.value).toMatch(/^[0-9a-f-]{36}$/i);
    expect((container.querySelector('input[name="address_id"]') as HTMLInputElement)?.value).toBe("a-1");
    expect((container.querySelector('input[name="shipping_method_id"]') as HTMLInputElement)?.value).toBe("s-1");
  });

  it("shows the total as subtotal + selected shipping", () => {
    render(<CheckoutForm subtotalCents={250000} currency="NGN" addresses={[addr]} shippingMethods={[ship]} />);
    // 250000 + 150000 = 400000 cents = ₦4,000.00
    expect(screen.getByText(/₦4,000\.00/)).toBeInTheDocument();
  });

  it("prompts to add an address when the book is empty", () => {
    render(<CheckoutForm subtotalCents={250000} currency="NGN" addresses={[]} shippingMethods={[ship]} />);
    expect(screen.getByText(/add a delivery address/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /place order/i })).not.toBeInTheDocument();
  });
});
