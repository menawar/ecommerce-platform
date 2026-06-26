import Link from "next/link";

export function Footer() {
  return (
    <footer
      style={{
        background: "var(--plt-green-deep)",
        color: "var(--plt-footer-text)",
        padding: "40px 24px 30px",
      }}
    >
      <div
        style={{
          maxWidth: 1180,
          margin: "0 auto",
          display: "grid",
          gridTemplateColumns: "1.6fr 1fr 1fr 1fr",
          gap: 26,
        }}
      >
        {/* Brand column */}
        <div>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 8,
              marginBottom: 12,
            }}
          >
            <svg
              width="24"
              height="24"
              viewBox="0 0 32 32"
              fill="none"
            >
              <path
                d="M1 25 L11 10 L17 18.5 L22 11 L31 25 Z"
                fill="#7fb56a"
              />
              <circle cx="24.5" cy="8" r="3.2" fill="#f3b73f" />
            </svg>
            <span style={{ fontWeight: 800, fontSize: 20, color: "#fff" }}>
              plateau<span style={{ color: "#e0894f" }}>.</span>
            </span>
          </div>
          <div
            style={{
              fontSize: 13,
              lineHeight: 1.7,
              color: "var(--plt-footer-muted)",
              maxWidth: 270,
            }}
          >
            Raw food materials, fresh from the Jos Plateau. Connecting Plateau
            farms and co-ops directly to your kitchen.
          </div>
          <div
            style={{
              fontSize: 12,
              color: "var(--plt-footer-dim)",
              marginTop: 14,
            }}
          >
            Farin Gada Market, Jos North, Plateau State
          </div>
        </div>

        {/* Shop column */}
        <div style={{ fontSize: 13, lineHeight: 2.3 }}>
          <b style={{ color: "#fff" }}>Shop</b>
          <br />
          <Link
            href="/products"
            style={{ color: "inherit", textDecoration: "none" }}
          >
            This week&apos;s harvest
          </Link>
          <br />
          <Link
            href="/products"
            style={{ color: "inherit", textDecoration: "none" }}
          >
            Bulk orders
          </Link>
          <br />
          <Link
            href="/products"
            style={{ color: "inherit", textDecoration: "none" }}
          >
            All produce
          </Link>
        </div>

        {/* Farms column */}
        <div style={{ fontSize: 13, lineHeight: 2.3 }}>
          <b style={{ color: "#fff" }}>Our farms</b>
          <br />
          Vom Farms
          <br />
          Riyom Growers
          <br />
          Barkin Ladi
        </div>

        {/* Help column */}
        <div style={{ fontSize: 13, lineHeight: 2.3 }}>
          <b style={{ color: "#fff" }}>Help</b>
          <br />
          <Link
            href="/orders"
            style={{ color: "inherit", textDecoration: "none" }}
          >
            Track order
          </Link>
          <br />
          Delivery areas
          <br />
          Contact
        </div>
      </div>

      {/* Copyright bar */}
      <div
        style={{
          maxWidth: 1180,
          margin: "18px auto 0",
          borderTop: "1px solid var(--plt-footer-rule)",
          paddingTop: 16,
          fontSize: 12,
          color: "var(--plt-footer-dim)",
          display: "flex",
          justifyContent: "space-between",
          flexWrap: "wrap",
          gap: 8,
        }}
      >
        <span>© 2026 Plateau · Home of Peace &amp; Tourism</span>
        <span>Naira (₦) · Delivering across Jos &amp; Plateau</span>
      </div>
    </footer>
  );
}
