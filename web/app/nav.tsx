import Link from "next/link";

import { isLoggedIn, currentRole } from "@/lib/session";
import { logoutAction } from "@/app/(auth)/actions";

// An async Server Component: it reads the session cookie on the server to decide
// which links to show. Reading cookies makes the layout render per-request
// (dynamic) — exactly what an auth-aware header needs.
export async function Nav() {
  const loggedIn = await isLoggedIn();
  // Role drives the Admin link; currentRole never throws (null when anon/expired).
  const isAdmin = loggedIn && (await currentRole()) === "admin";

  // Try to get cart count when logged in
  let cartCount = 0;
  if (loggedIn) {
    try {
      const { getCart } = await import("@/lib/cart");
      const cart = await getCart();
      cartCount = cart.items.reduce((sum, item) => sum + item.quantity, 0);
    } catch {
      // Cart fetch failed — show 0 count silently
    }
  }

  return (
    <header>
      {/* ── Announcement Bar ──────────────────────────────────────────────── */}
      <div
        className="text-center text-white"
        style={{
          background: "var(--plt-terracotta)",
          fontSize: "12.5px",
          fontWeight: 600,
          padding: "7px 16px",
          letterSpacing: ".01em",
        }}
      >
        Fresh from the Jos Plateau &nbsp;·&nbsp; Home of Peace &amp; Tourism
        &nbsp;·&nbsp; Free delivery on bulk orders over ₦50,000
      </div>

      {/* ── Utility Bar ───────────────────────────────────────────────────── */}
      <div
        className="text-white"
        style={{
          background: "var(--plt-green-deep)",
          display: "flex",
          alignItems: "center",
          gap: 18,
          padding: "12px 24px",
          flexWrap: "wrap",
        }}
      >
        {/* Logo */}
        <Link
          href="/"
          className="flex items-center gap-2 no-underline text-white"
          style={{ textDecoration: "none" }}
        >
          <svg
            width="27"
            height="27"
            viewBox="0 0 32 32"
            fill="none"
          >
            <path
              d="M1 25 L11 10 L17 18.5 L22 11 L31 25 Z"
              fill="#7fb56a"
            />
            <path d="M1 25 L11 10 L15.5 16.4 L8 25 Z" fill="#5f9a4d" />
            <circle cx="24.5" cy="8" r="3.2" fill="#f3b73f" />
          </svg>
          <span style={{ fontWeight: 800, fontSize: 22 }}>
            plateau<span style={{ color: "#e0894f" }}>.</span>
          </span>
        </Link>

        {/* Deliver to */}
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            lineHeight: 1.1,
            fontSize: 12,
          }}
        >
          <span style={{ color: "#aab2bd" }}>Deliver to</span>
          <span style={{ fontWeight: 700 }}>Jos, Plateau</span>
        </div>

        {/* Search bar */}
        <div
          style={{
            flex: 1,
            minWidth: 220,
            display: "flex",
            alignItems: "stretch",
            borderRadius: 6,
            overflow: "hidden",
          }}
        >
          <form
            action="/products"
            style={{ display: "flex", flex: 1 }}
          >
            <input
              type="search"
              name="q"
              placeholder="Search fresh produce"
              style={{
                border: 0,
                flex: 1,
                padding: "11px 14px",
                fontSize: 14,
                color: "var(--plt-text)",
              }}
            />
            <button
              type="submit"
              style={{
                background: "var(--plt-terracotta)",
                border: 0,
                padding: "0 18px",
                cursor: "pointer",
                display: "flex",
                alignItems: "center",
              }}
            >
              <svg
                width="19"
                height="19"
                viewBox="0 0 24 24"
                fill="none"
                stroke="#fff"
                strokeWidth="2.2"
              >
                <circle cx="11" cy="11" r="7" />
                <line
                  x1="16.5"
                  y1="16.5"
                  x2="21"
                  y2="21"
                  strokeLinecap="round"
                />
              </svg>
            </button>
          </form>
        </div>

        {/* Account */}
        <Link
          href={loggedIn ? "/account" : "/login"}
          style={{
            display: "flex",
            flexDirection: "column",
            lineHeight: 1.15,
            fontSize: 12,
            textDecoration: "none",
            color: "white",
          }}
        >
          <span style={{ color: "#aab2bd" }}>
            {loggedIn ? "Your" : "Hello, sign in"}
          </span>
          <span style={{ fontWeight: 700 }}>Account &amp; Orders</span>
        </Link>

        {/* Cart */}
        <Link
          href="/cart"
          style={{
            display: "flex",
            alignItems: "flex-end",
            gap: 7,
            position: "relative",
            textDecoration: "none",
            color: "white",
          }}
        >
          <svg
            width="26"
            height="26"
            viewBox="0 0 24 24"
            fill="none"
            stroke="#fff"
            strokeWidth="1.7"
          >
            <circle cx="9" cy="20" r="1.3" />
            <circle cx="18" cy="20" r="1.3" />
            <path
              d="M2 3h3l2.4 12h11l2-9H6.4"
              strokeLinejoin="round"
              strokeLinecap="round"
            />
          </svg>
          <span style={{ fontWeight: 800, fontSize: 14 }}>Cart</span>
          {cartCount > 0 && (
            <span
              style={{
                position: "absolute",
                top: -6,
                left: 14,
                background: "var(--plt-terracotta)",
                color: "#fff",
                fontSize: 11,
                fontWeight: 800,
                borderRadius: 9,
                padding: "0 6px",
              }}
            >
              {cartCount}
            </span>
          )}
        </Link>

        {/* Auth links (visible on mobile / narrow screens) */}
        {loggedIn && (
          <form action={logoutAction}>
            <button
              className="text-white"
              style={{
                background: "none",
                border: "1px solid rgba(255,255,255,.3)",
                borderRadius: 6,
                padding: "6px 14px",
                fontSize: 12,
                fontWeight: 600,
                cursor: "pointer",
              }}
            >
              Log out
            </button>
          </form>
        )}
      </div>

      {/* ── Category Nav ──────────────────────────────────────────────────── */}
      <nav
        style={{
          background: "var(--plt-green-mid)",
          color: "#dfe7e0",
          display: "flex",
          alignItems: "center",
          gap: "6px 22px",
          padding: "9px 24px",
          fontSize: 13,
          flexWrap: "wrap",
        }}
      >
        <Link
          href="/products"
          style={{ color: "#dfe7e0", textDecoration: "none" }}
        >
          All produce
        </Link>
        {loggedIn && (
          <>
            <Link
              href="/cart"
              style={{ color: "#dfe7e0", textDecoration: "none" }}
            >
              Cart
            </Link>
            <Link
              href="/orders"
              style={{ color: "#dfe7e0", textDecoration: "none" }}
            >
              Orders
            </Link>
            <Link
              href="/account"
              style={{ color: "#dfe7e0", textDecoration: "none" }}
            >
              Account
            </Link>
            {isAdmin && (
              <Link
                href="/admin/products"
                style={{ color: "#ffd98a", textDecoration: "none", fontWeight: 700 }}
              >
                Admin
              </Link>
            )}
          </>
        )}
        {!loggedIn && (
          <>
            <Link
              href="/login"
              style={{ color: "#dfe7e0", textDecoration: "none" }}
            >
              Sign in
            </Link>
            <Link
              href="/register"
              style={{ color: "#dfe7e0", textDecoration: "none" }}
            >
              Register
            </Link>
          </>
        )}
      </nav>
    </header>
  );
}
