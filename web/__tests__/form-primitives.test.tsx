import { describe, it, expect, afterEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";

import { Input } from "@/components/ui/input";
import { Field } from "@/components/ui/field";

describe("Input", () => {
  afterEach(cleanup);

  it("sets aria-invalid when invalid", () => {
    render(<Input aria-label="Coupon" invalid />);
    expect(screen.getByLabelText("Coupon")).toHaveAttribute("aria-invalid", "true");
  });

  it("has no aria-invalid when valid", () => {
    render(<Input aria-label="Email" />);
    expect(screen.getByLabelText("Email")).not.toHaveAttribute("aria-invalid");
  });
});

describe("Field", () => {
  afterEach(cleanup);

  it("associates the label with the control via htmlFor/id", () => {
    render(
      <Field label="Email" htmlFor="email">
        <Input id="email" />
      </Field>,
    );
    // getByLabelText resolves the control through the <label for> association.
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
  });

  it("shows the error instead of the hint when both are set", () => {
    render(
      <Field label="Coupon" htmlFor="c" hint="Enter a code" error="Expired">
        <Input id="c" />
      </Field>,
    );
    expect(screen.getByText("Expired")).toBeInTheDocument();
    expect(screen.queryByText("Enter a code")).not.toBeInTheDocument();
  });

  it("wires aria-invalid + aria-describedby onto the control when in error", () => {
    render(
      <Field label="Coupon" htmlFor="c" error="Expired">
        <Input id="c" />
      </Field>,
    );
    const input = screen.getByLabelText("Coupon");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAttribute("aria-describedby", "c-message");
    // the message element carries that id
    expect(screen.getByText("Expired")).toHaveAttribute("id", "c-message");
  });
});
