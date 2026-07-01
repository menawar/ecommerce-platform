import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";

import { Button } from "@/components/ui/button";
import { Rating } from "@/components/ui/rating";
import { Badge } from "@/components/ui/badge";

describe("Button", () => {
  afterEach(cleanup);

  it("renders children and handles clicks", () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>Add to cart</Button>);
    fireEvent.click(screen.getByRole("button", { name: /add to cart/i }));
    expect(onClick).toHaveBeenCalledOnce();
  });

  it("is disabled and busy while loading, and does not fire onClick", () => {
    const onClick = vi.fn();
    render(
      <Button loading onClick={onClick}>
        Save
      </Button>,
    );
    const btn = screen.getByRole("button", { name: /save/i });
    expect(btn).toBeDisabled();
    expect(btn).toHaveAttribute("aria-busy", "true");
    fireEvent.click(btn);
    expect(onClick).not.toHaveBeenCalled();
  });

  it("merges a caller className over the variant classes", () => {
    render(<Button className="w-40">Wide</Button>);
    expect(screen.getByRole("button", { name: /wide/i })).toHaveClass("w-40");
  });

  it('defaults to type="button" but lets callers opt into submit', () => {
    const { rerender } = render(<Button>Default</Button>);
    expect(screen.getByRole("button")).toHaveAttribute("type", "button");
    rerender(
      <Button type="submit">Go</Button>,
    );
    expect(screen.getByRole("button")).toHaveAttribute("type", "submit");
  });
});

describe("Rating", () => {
  afterEach(cleanup);

  it("exposes an accessible label with value and review count", () => {
    render(<Rating value={4.5} count={128} />);
    expect(screen.getByLabelText(/rated 4\.5 out of 5, 128 reviews/i)).toBeInTheDocument();
  });

  it("clamps out-of-range values", () => {
    render(<Rating value={9} />);
    expect(screen.getByLabelText(/rated 5\.0 out of 5/i)).toBeInTheDocument();
  });
});

describe("Badge", () => {
  afterEach(cleanup);

  it("renders its label", () => {
    render(<Badge variant="danger">Low stock</Badge>);
    expect(screen.getByText("Low stock")).toBeInTheDocument();
  });
});
