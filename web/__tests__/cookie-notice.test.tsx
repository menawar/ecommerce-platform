import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";

import { CookieNotice } from "@/app/cookie-notice";

describe("CookieNotice", () => {
  beforeEach(() => localStorage.clear());
  afterEach(cleanup);

  it("shows the notice when not previously acknowledged", () => {
    render(<CookieNotice />);
    expect(screen.getByRole("region", { name: /cookie notice/i })).toBeInTheDocument();
  });

  it("dismisses and remembers the choice", () => {
    render(<CookieNotice />);
    fireEvent.click(screen.getByRole("button", { name: /got it/i }));
    expect(screen.queryByRole("region", { name: /cookie notice/i })).not.toBeInTheDocument();
    expect(localStorage.getItem("plt-cookie-ack")).toBe("1");
  });

  it("stays hidden when already acknowledged", () => {
    localStorage.setItem("plt-cookie-ack", "1");
    render(<CookieNotice />);
    expect(screen.queryByRole("region", { name: /cookie notice/i })).not.toBeInTheDocument();
  });
});
