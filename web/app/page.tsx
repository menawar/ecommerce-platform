import Link from "next/link";
import { listProducts } from "@/lib/gateway";
import { formatPrice } from "@/lib/format";

// The home page is a Server Component (the App Router default). No "use client"
// here — it renders to HTML on the server and ships zero JavaScript for this view.
export default async function Home() {
  // Fetch products for the homepage display
  let products: Awaited<ReturnType<typeof listProducts>>["products"] = [];
  try {
    const result = await listProducts({ page: 1, pageSize: 12 });
    products = result.products;
  } catch {
    // Silently fall back to empty grid if gateway is unavailable
  }

  // Split into deals (first 6) and fresh (all)
  const deals = products.slice(0, 6);
  const fresh = products;

  return (
    <main
      style={{
        maxWidth: 1180,
        margin: "0 auto",
        padding: "18px 20px 50px",
      }}
    >
      {/* ── Hero Banner ─────────────────────────────────────────────────── */}
      <div
        style={{
          position: "relative",
          borderRadius: "var(--plt-radius-xl)",
          overflow: "hidden",
          minHeight: 330,
          display: "flex",
          alignItems: "center",
          padding: "0 56px",
          background:
            "linear-gradient(180deg, var(--plt-hero-start) 0%, var(--plt-hero-end) 100%)",
        }}
      >
        {/* Topographic landscape SVG */}
        <svg
          viewBox="0 0 640 330"
          preserveAspectRatio="xMidYMax slice"
          style={{
            position: "absolute",
            inset: 0,
            width: "100%",
            height: "100%",
          }}
        >
          <circle cx="505" cy="88" r="46" fill="#f3b73f" opacity="0.9" />
          <path
            d="M0 248 Q140 178 300 216 T640 198 V330 H0 Z"
            fill="#d6e4c6"
          />
          <path
            d="M0 270 Q170 208 360 246 T640 234 V330 H0 Z"
            fill="#a7c98c"
          />
          <path
            d="M0 296 Q200 248 410 278 T640 272 V330 H0 Z"
            fill="#5f9a4d"
          />
          <path
            d="M0 318 Q220 290 460 308 T640 302 V330 H0 Z"
            fill="#3c6b34"
          />
        </svg>

        <div
          style={{
            position: "relative",
            zIndex: 1,
            maxWidth: 470,
          }}
        >
          <div
            style={{
              fontSize: 13,
              fontWeight: 800,
              color: "var(--plt-terracotta)",
              letterSpacing: ".08em",
              textTransform: "uppercase",
            }}
          >
            Jos Plateau · Home of Peace &amp; Tourism
          </div>
          <h1
            style={{
              fontSize: 44,
              fontWeight: 800,
              lineHeight: 1.05,
              margin: "10px 0 16px",
              letterSpacing: "-.02em",
              color: "var(--plt-green-deep)",
            }}
          >
            Raw food, straight from the Plateau.
          </h1>
          <p
            style={{
              fontSize: 16,
              color: "var(--plt-text-secondary)",
              marginBottom: 22,
            }}
          >
            Irish potatoes, acha, sweet tomatoes — grown in the cool Jos
            highlands, harvested to order.
          </p>
          <Link href="/products" className="plt-btn-hero">
            Shop the harvest
          </Link>
        </div>
      </div>

      {/* ── Top Harvest Deals ─────────────────────────────────────────────── */}
      {deals.length > 0 && (
        <div className="plt-card" style={{ marginTop: 18 }}>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              marginBottom: 18,
            }}
          >
            <div style={{ fontSize: 20, fontWeight: 800 }}>
              Top harvest deals
            </div>
            <Link
              href="/products"
              style={{
                fontSize: 13,
                color: "var(--plt-terracotta)",
                fontWeight: 700,
                textDecoration: "none",
              }}
            >
              See all →
            </Link>
          </div>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(186px, 1fr))",
              gap: 16,
            }}
          >
            {deals.map((p) => (
              <Link
                key={p.id}
                href={`/products/${p.id}`}
                style={{
                  background: "var(--plt-card)",
                  border: "1px solid var(--plt-border)",
                  borderRadius: "var(--plt-radius-md)",
                  padding: 14,
                  display: "flex",
                  flexDirection: "column",
                  textDecoration: "none",
                  color: "var(--plt-text)",
                  transition: "box-shadow 0.2s",
                }}
              >
                <div className="plt-img-placeholder">
                  <span
                    style={{
                      fontFamily: "monospace",
                      fontSize: 10,
                      color: "var(--plt-text-muted)",
                    }}
                  >
                    {p.sku}
                  </span>
                </div>
                <div
                  style={{
                    fontSize: 13,
                    lineHeight: 1.3,
                    margin: "11px 0 6px",
                    height: 34,
                    overflow: "hidden",
                  }}
                >
                  {p.name}
                </div>
                <div
                  style={{
                    fontSize: 12,
                    color: "var(--plt-star)",
                    letterSpacing: ".5px",
                  }}
                >
                  ★★★★★{" "}
                  <span style={{ color: "var(--plt-terracotta)" }}>
                    ({p.available})
                  </span>
                </div>
                <div
                  style={{
                    display: "flex",
                    alignItems: "baseline",
                    gap: 7,
                    margin: "7px 0 3px",
                  }}
                >
                  <span style={{ fontSize: 17, fontWeight: 800 }}>
                    {formatPrice(p.price_cents, p.currency)}
                  </span>
                </div>
                <div
                  style={{
                    fontSize: 11,
                    color: "var(--plt-green-text)",
                    fontWeight: 700,
                    marginBottom: 11,
                  }}
                >
                  {p.available > 0 ? "Delivered this week" : "Out of stock"}
                </div>
              </Link>
            ))}
          </div>
        </div>
      )}

      {/* ── Fresh For You ─────────────────────────────────────────────────── */}
      {fresh.length > 0 && (
        <div className="plt-card" style={{ marginTop: 18 }}>
          <div style={{ fontSize: 20, fontWeight: 800, marginBottom: 18 }}>
            Fresh for you
          </div>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(186px, 1fr))",
              gap: 16,
            }}
          >
            {fresh.map((p) => (
              <Link
                key={p.id}
                href={`/products/${p.id}`}
                style={{
                  background: "var(--plt-card)",
                  border: "1px solid var(--plt-border)",
                  borderRadius: "var(--plt-radius-md)",
                  padding: 14,
                  display: "flex",
                  flexDirection: "column",
                  textDecoration: "none",
                  color: "var(--plt-text)",
                  transition: "box-shadow 0.2s",
                }}
              >
                <div className="plt-img-placeholder">
                  {p.available > 0 && (
                    <span className="plt-badge plt-badge-dark">In stock</span>
                  )}
                  <span
                    style={{
                      fontFamily: "monospace",
                      fontSize: 10,
                      color: "var(--plt-text-muted)",
                    }}
                  >
                    {p.sku}
                  </span>
                </div>
                <div
                  style={{
                    fontSize: 13,
                    lineHeight: 1.3,
                    margin: "11px 0 6px",
                    height: 34,
                    overflow: "hidden",
                  }}
                >
                  {p.name}
                </div>
                <div
                  style={{
                    fontSize: 12,
                    color: "var(--plt-star)",
                    letterSpacing: ".5px",
                  }}
                >
                  ★★★★★{" "}
                  <span style={{ color: "var(--plt-terracotta)" }}>
                    ({p.available})
                  </span>
                </div>
                <div
                  style={{ fontSize: 17, fontWeight: 800, margin: "7px 0 11px" }}
                >
                  {formatPrice(p.price_cents, p.currency)}
                </div>
              </Link>
            ))}
          </div>
        </div>
      )}

      {/* ── Empty state ───────────────────────────────────────────────────── */}
      {products.length === 0 && (
        <div
          className="plt-card"
          style={{ marginTop: 18, textAlign: "center", padding: "60px 20px" }}
        >
          <div style={{ fontSize: 18, fontWeight: 700, marginBottom: 6 }}>
            Welcome to Plateau
          </div>
          <p
            style={{
              fontSize: 14,
              color: "var(--plt-text-secondary)",
              marginBottom: 20,
            }}
          >
            Browse this week&apos;s harvest from the Jos Plateau.
          </p>
          <Link href="/products" className="plt-btn-primary-lg">
            Browse produce
          </Link>
        </div>
      )}
    </main>
  );
}
