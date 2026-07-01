import { describe, it, expect, afterEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";

import { Container } from "@/components/ui/container";

describe("Container", () => {
  afterEach(cleanup);

  it("centers with the default (xl) max width and responsive padding", () => {
    render(<Container data-testid="c">hi</Container>);
    const el = screen.getByTestId("c");
    expect(el.className).toContain("mx-auto");
    expect(el.className).toContain("max-w-[1180px]");
  });

  it("applies the chosen size", () => {
    render(
      <Container size="sm" data-testid="c">
        hi
      </Container>,
    );
    expect(screen.getByTestId("c").className).toContain("max-w-[480px]");
  });

  it("renders as the given element via `as`", () => {
    render(
      <Container as="main" data-testid="c">
        hi
      </Container>,
    );
    expect(screen.getByTestId("c").tagName).toBe("MAIN");
  });
});
