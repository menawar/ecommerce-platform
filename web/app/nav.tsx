import Link from "next/link";

import { isLoggedIn, getMe } from "@/lib/session";
import { logoutAction } from "@/app/(auth)/actions";
import { MobileMenu } from "./mobile-menu";

// An async Server Component: it reads the session cookie on the server to decide
// which links to show. Reading cookies makes the layout render per-request
// (dynamic) — exactly what an auth-aware header needs. The mobile hamburger/drawer
// is a small client island (MobileMenu); everything else stays server-rendered.
export async function Nav() {
  const loggedIn = await isLoggedIn();
  // One /me round-trip serves both the Admin link (role) and the verify banner
  // (email_verified). getMe throws on an expired/invalid token, so swallow it —
  // the header should still render for a lapsed session (links just go anonymous).
  let me: { role: string; email_verified: boolean } | null = null;
  if (loggedIn) {
    try {
      me = await getMe();
    } catch {
      me = null;
    }
  }
  const isAdmin = me?.role === "admin";
  const needsVerify = me !== null && !me.email_verified;

  let cartCount = 0;
  if (loggedIn) {
    try {
      const { getCart } = await import("@/lib/cart");
      const cart = await getCart();
      cartCount = cart.items.reduce((sum, item) => sum + item.quantity, 0);
    } catch {
      // Cart fetch failed — show 0 count silently.
    }
  }

  const categoryLink = "text-[#dfe7e0] no-underline hover:text-white";
  const adminLink = "font-bold text-[#ffd98a] no-underline hover:brightness-110";

  return (
    <header>
      {/* Announcement bar */}
      <div className="bg-accent px-4 py-1.5 text-center text-[12.5px] font-semibold tracking-[.01em] text-white">
        Fresh from the Jos Plateau &nbsp;·&nbsp; Home of Peace &amp; Tourism &nbsp;·&nbsp; Free
        delivery on bulk orders over ₦50,000
      </div>

      {/* Verify-email banner */}
      {needsVerify && (
        <div className="bg-gold px-4 py-2 text-center text-[13px] font-semibold text-[#3a2c00]">
          Verify your email to place orders.{" "}
          <Link href="/verify-email" className="text-[#3a2c00] underline">
            Resend the link
          </Link>
        </div>
      )}

      {/* Main bar */}
      <div className="flex flex-wrap items-center gap-3 bg-brand-deep px-4 py-3 text-white sm:gap-4 sm:px-6">
        <MobileMenu loggedIn={loggedIn} isAdmin={isAdmin} />

        {/* Logo */}
        <Link href="/" className="flex items-center gap-2 text-white no-underline">
          <svg width="27" height="27" viewBox="0 0 32 32" fill="none" aria-hidden>
            <path d="M1 25 L11 10 L17 18.5 L22 11 L31 25 Z" fill="#7fb56a" />
            <path d="M1 25 L11 10 L15.5 16.4 L8 25 Z" fill="#5f9a4d" />
            <circle cx="24.5" cy="8" r="3.2" fill="#f3b73f" />
          </svg>
          <span className="text-[22px] font-extrabold">
            plateau<span className="text-[#e0894f]">.</span>
          </span>
        </Link>

        {/* Deliver to (wide screens only) */}
        <div className="hidden flex-col text-xs leading-tight lg:flex">
          <span className="text-[#aab2bd]">Deliver to</span>
          <span className="font-bold">Jos, Plateau</span>
        </div>

        {/* Search */}
        <form action="/products" className="flex min-w-0 flex-1 basis-full overflow-hidden rounded-md bg-white sm:basis-0">
          <input
            type="search"
            name="q"
            placeholder="Search fresh produce"
            aria-label="Search products"
            className="min-w-0 flex-1 border-0 px-3.5 py-2.5 text-sm text-fg outline-none"
          />
          <button
            type="submit"
            aria-label="Search"
            className="flex items-center bg-accent px-4 text-white"
          >
            <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2.2" aria-hidden>
              <circle cx="11" cy="11" r="7" />
              <line x1="16.5" y1="16.5" x2="21" y2="21" strokeLinecap="round" />
            </svg>
          </button>
        </form>

        {/* Account (md+) */}
        <Link
          href={loggedIn ? "/account" : "/login"}
          className="hidden flex-col text-xs leading-tight text-white no-underline md:flex"
        >
          <span className="text-[#aab2bd]">{loggedIn ? "Your" : "Hello, sign in"}</span>
          <span className="font-bold">Account &amp; Orders</span>
        </Link>

        {/* Cart */}
        <Link href="/cart" className="relative flex items-end gap-1.5 text-white no-underline">
          <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="1.7" aria-hidden>
            <circle cx="9" cy="20" r="1.3" />
            <circle cx="18" cy="20" r="1.3" />
            <path d="M2 3h3l2.4 12h11l2-9H6.4" strokeLinejoin="round" strokeLinecap="round" />
          </svg>
          <span className="hidden text-sm font-extrabold sm:inline">Cart</span>
          {cartCount > 0 && (
            <span className="absolute -top-1.5 left-3.5 rounded-full bg-accent px-1.5 text-[11px] font-extrabold text-white">
              {cartCount}
            </span>
          )}
        </Link>

        {/* Log out (md+; mobile has it in the drawer) */}
        {loggedIn && (
          <form action={logoutAction} className="hidden md:block">
            <button className="rounded-md border border-white/30 px-3.5 py-1.5 text-xs font-semibold text-white hover:bg-white/10">
              Log out
            </button>
          </form>
        )}
      </div>

      {/* Category nav (desktop; mobile uses the drawer) */}
      <nav
        aria-label="Product categories"
        className="hidden flex-wrap items-center gap-x-6 gap-y-1.5 bg-brand-mid px-6 py-2.5 text-[13px] text-[#dfe7e0] md:flex"
      >
        <Link href="/products" className={categoryLink}>
          All produce
        </Link>
        {loggedIn ? (
          <>
            <Link href="/cart" className={categoryLink}>
              Cart
            </Link>
            <Link href="/orders" className={categoryLink}>
              Orders
            </Link>
            <Link href="/account" className={categoryLink}>
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
            <Link href="/login" className={categoryLink}>
              Sign in
            </Link>
            <Link href="/register" className={categoryLink}>
              Register
            </Link>
          </>
        )}
      </nav>
    </header>
  );
}
