import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";

import { Drawer } from "@/components/ui/drawer";

function open(props: Partial<Parameters<typeof Drawer>[0]> = {}) {
  const onClose = vi.fn();
  render(
    <Drawer open onClose={onClose} title="Menu" {...props}>
      <a href="#one">One</a>
    </Drawer>,
  );
  return { onClose };
}

describe("Drawer", () => {
  afterEach(cleanup);

  it("renders a labelled modal dialog when open", () => {
    open();
    const dialog = screen.getByRole("dialog", { name: "Menu" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
  });

  it("renders nothing when closed", () => {
    render(
      <Drawer open={false} onClose={() => {}} title="Menu">
        <a href="#one">One</a>
      </Drawer>,
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("closes on Escape, the close button, and overlay click", () => {
    const { onClose } = open();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: /close/i }));
    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it("locks body scroll while open and restores it on unmount", () => {
    const { rerender } = render(
      <Drawer open onClose={() => {}} title="Menu">
        x
      </Drawer>,
    );
    expect(document.body.style.overflow).toBe("hidden");
    rerender(
      <Drawer open={false} onClose={() => {}} title="Menu">
        x
      </Drawer>,
    );
    expect(document.body.style.overflow).not.toBe("hidden");
  });
});
