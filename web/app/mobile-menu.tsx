"use client";

import { useState } from "react";
import Link from "next/link";

import { Drawer } from "@/components/ui/drawer";
import { Button } from "@/components/ui/button";
import { logoutAction } from "@/app/(auth)/actions";

// The mobile navigation island: a hamburger (shown only below md) that opens the
// Drawer with the same links the desktop category bar shows. The server Nav passes
// the session-derived flags in. Clicking a link closes the drawer.
export function MobileMenu({ loggedIn, isAdmin }: { loggedIn: boolean; isAdmin: boolean }) {
  const [open, setOpen] = useState(false);
  const link = "rounded-md px-2 py-2.5 text-fg hover:bg-surface";
  const adminLink = link + " font-semibold text-accent";

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-label={open ? "Close menu" : "Open menu"}
        aria-expanded={open}
        className="-ml-1 rounded-md p-1.5 text-white hover:bg-white/10 md:hidden"
      >
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
          <line x1="3" y1="6" x2="21" y2="6" strokeLinecap="round" />
          <line x1="3" y1="12" x2="21" y2="12" strokeLinecap="round" />
          <line x1="3" y1="18" x2="21" y2="18" strokeLinecap="round" />
        </svg>
      </button>

      <Drawer open={open} onClose={() => setOpen(false)} title="Menu" side="left">
        {/* Close the drawer when any link is chosen. */}
        <nav className="flex flex-col gap-1 text-sm" onClick={() => setOpen(false)}>
          <Link href="/products" className={link}>
            All produce
          </Link>
          {loggedIn ? (
            <>
              <Link href="/cart" className={link}>
                Cart
              </Link>
              <Link href="/orders" className={link}>
                Orders
              </Link>
              <Link href="/account" className={link}>
                Account
              </Link>
              {isAdmin && (
                <>
                  <Link href="/admin/products" className={adminLink}>
                    Admin
                  </Link>
                  <Link href="/admin/shipping" className={adminLink}>
                    Shipping
                  </Link>
                  <Link href="/admin/orders" className={adminLink}>
                    Fulfillment
                  </Link>
                </>
              )}
            </>
          ) : (
            <>
              <Link href="/login" className={link}>
                Sign in
              </Link>
              <Link href="/register" className={link}>
                Register
              </Link>
            </>
          )}
        </nav>

        {loggedIn && (
          <form action={logoutAction} className="mt-4 border-t border-border pt-4">
            <Button type="submit" variant="outline" fullWidth>
              Log out
            </Button>
          </form>
        )}
      </Drawer>
    </>
  );
}
