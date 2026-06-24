import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";

// ---- Mocks ----
// CheckoutButton imports from ./actions (a "use server" file) which can't run in
// jsdom. We mock the module so the component renders without hitting Next internals.
// The POINT of this test is the idempotency key, not the Server Action itself.
vi.mock("@/app/checkout/actions", () => ({
  placeOrderAction: vi.fn(),
}));

// React's useActionState is provided by the react-dom/server integration in Next.
// In plain Vitest+jsdom it doesn't exist, so we stub it to return [state, action, pending].
vi.mock("react", async () => {
  const actual = await vi.importActual<typeof import("react")>("react");
  return {
    ...actual,
    useActionState: (
      _action: unknown,
      initialState: unknown,
    ) => {
      // Returns [state, formAction, isPending] — the component just needs
      // a no-op formAction and pending=false so it renders the button.
      return [initialState, vi.fn(), false];
    },
  };
});

import { CheckoutButton } from "@/app/checkout/checkout-button";

describe("CheckoutButton", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders a Place order button", () => {
    render(<CheckoutButton />);
    expect(screen.getByRole("button", { name: /place order/i })).toBeInTheDocument();
  });

  it("generates an idempotency key in a hidden input", () => {
    const { container } = render(<CheckoutButton />);
    const hidden = container.querySelector('input[name="idempotency_key"]');
    expect(hidden).toBeInTheDocument();
    // The key should be a valid UUID (crypto.randomUUID format)
    const key = (hidden as HTMLInputElement).value;
    expect(key).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/,
    );
  });

  it("generates the SAME key across re-renders (stable within one mount)", () => {
    // This is the core idempotency guarantee: double-clicks / re-renders within
    // the same page load must reuse the same key, so the saga deduplicates.
    const { container, rerender } = render(<CheckoutButton />);
    const key1 = (
      container.querySelector('input[name="idempotency_key"]') as HTMLInputElement
    ).value;

    rerender(<CheckoutButton />);
    const key2 = (
      container.querySelector('input[name="idempotency_key"]') as HTMLInputElement
    ).value;

    expect(key1).toBe(key2);
  });

  it("generates a DIFFERENT key on a fresh mount (new checkout attempt)", () => {
    // Unmount + remount simulates the user navigating away and back — a new
    // deliberate checkout attempt that SHOULD get its own idempotency key.
    const { container, unmount } = render(<CheckoutButton />);
    const key1 = (
      container.querySelector('input[name="idempotency_key"]') as HTMLInputElement
    ).value;
    unmount();

    const { container: container2 } = render(<CheckoutButton />);
    const key2 = (
      container2.querySelector('input[name="idempotency_key"]') as HTMLInputElement
    ).value;

    expect(key1).not.toBe(key2);
  });
});
