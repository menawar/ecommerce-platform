import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";

// The mobile menu imports the server logout action; replace the whole module so its
// server-only deps don't load in jsdom.
vi.mock("@/app/(auth)/actions", () => ({ logoutAction: vi.fn() }));

import { MobileMenu } from "@/app/mobile-menu";

function openMenu() {
  fireEvent.click(screen.getByRole("button", { name: /open menu/i }));
}

describe("MobileMenu", () => {
  afterEach(cleanup);

  it("shows sign-in/register links when logged out (no account links)", () => {
    render(<MobileMenu loggedIn={false} isAdmin={false} />);
    openMenu();
    expect(screen.getByRole("link", { name: "Sign in" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Register" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Orders" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /log out/i })).not.toBeInTheDocument();
  });

  it("shows account links + logout when logged in, but no admin links for non-admins", () => {
    render(<MobileMenu loggedIn isAdmin={false} />);
    openMenu();
    expect(screen.getByRole("link", { name: "Orders" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /log out/i })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Fulfillment" })).not.toBeInTheDocument();
  });

  it("shows admin links for admins", () => {
    render(<MobileMenu loggedIn isAdmin />);
    openMenu();
    expect(screen.getByRole("link", { name: "Admin" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Fulfillment" })).toBeInTheDocument();
  });
});
