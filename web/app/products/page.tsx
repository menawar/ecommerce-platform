import type { Metadata } from "next";
import Link from "next/link";
import { listProducts, GatewayError } from "@/lib/gateway";
import { formatPrice } from "@/lib/format";
import { ErrorPanel } from "../error-panel";
import { SortSelect } from "./sort-select";

export const metadata: Metadata = {
  title: "Shop",
  description: "Browse this week's harvest — raw food materials fresh from the Jos Plateau.",
  alternates: { canonical: "/products" },
};

const PAGE_SIZE = 12;

// value = the backend sort key (server normalizes "featured"/unknown -> newest
// first). "Top rated" is intentionally absent: there's no ratings data yet, so it
// would be another dropdown that does nothing.
const SORT_OPTIONS = [
  { value: "featured", label: "Featured" },
  { value: "price_asc", label: "Price: Low to High" },
  { value: "price_desc", label: "Price: High to Low" },
];

// A Server Component: it runs on the server, awaits the gateway call directly, and
// streams HTML to the browser. searchParams is async in the App Router (Next 16).
export default async function ProductsPage({
  searchParams,
}: {
  searchParams: Promise<{ q?: string; page?: string; sort?: string }>;
}) {
  const sp = await searchParams;
  const q = sp.q ?? "";
  const page = Math.max(1, Number(sp.page) || 1);
  const sort = sp.sort ?? "featured";

  // Catch the gateway error HERE rather than letting it throw to error.tsx: a
  // Server Component can render the requestId into HTML directly, whereas the
  // error boundary would only receive a redacted error (no requestId, no message
  // in production). Anything that ISN'T a GatewayError is genuinely unexpected —
  // rethrow it and let the error boundary catch it.
  let products, total;
  try {
    ({ products, total } = await listProducts({ q, page, sort, pageSize: PAGE_SIZE }));
  } catch (err) {
    if (err instanceof GatewayError) {
      return (
        <main style={{ maxWidth: 1180, margin: "0 auto", padding: "16px 20px 50px" }}>
          <h1 style={{ fontSize: 20, fontWeight: 800 }}>Products</h1>
          <div style={{ marginTop: 24 }}>
            <ErrorPanel
              message={`Couldn't load products: ${err.message}`}
              requestId={err.requestId}
            />
          </div>
        </main>
      );
    }
    throw err;
  }
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <main style={{ maxWidth: 1180, margin: "0 auto", padding: "16px 20px 50px" }}>
      {/* Breadcrumb */}
      <div
        style={{
          fontSize: 13,
          color: "var(--plt-text-secondary)",
          marginBottom: 16,
        }}
      >
        <Link href="/" style={{ color: "inherit", textDecoration: "none" }}>
          Home
        </Link>{" "}
        ›{" "}
        <b style={{ color: "var(--plt-text)" }}>
          {q ? `Search: "${q}"` : "All produce"}
        </b>{" "}
        &nbsp;·&nbsp;{" "}
        <b style={{ color: "var(--plt-text)" }}>{total}</b> results
      </div>

      <div
        style={{
          display: "flex",
          gap: 20,
          alignItems: "flex-start",
          flexWrap: "wrap",
        }}
      >
        {/* ── Sidebar ──────────────────────────────────────────────────── */}
        <div
          style={{
            width: 230,
            flex: "0 0 230px",
            background: "var(--plt-card)",
            borderRadius: "var(--plt-radius-md)",
            padding: 20,
          }}
        >
          {/* Search form */}
          <div style={{ fontSize: 14, fontWeight: 800, marginBottom: 10 }}>
            Search
          </div>
          <form
            action="/products"
            style={{
              display: "flex",
              flexDirection: "column",
              gap: 9,
              paddingBottom: 18,
              borderBottom: "1px solid var(--plt-border-heavy)",
            }}
          >
            {/* Carry the active sort through a search so the two filters compose. */}
            {sort && sort !== "featured" && (
              <input type="hidden" name="sort" value={sort} />
            )}
            <input
              type="search"
              name="q"
              defaultValue={q}
              placeholder="Search products…"
              className="plt-input"
              style={{ fontSize: 13 }}
            />
            <button className="plt-btn-primary" type="submit">
              Search
            </button>
          </form>

          {/* Customer rating */}
          <div
            style={{
              fontSize: 14,
              fontWeight: 800,
              padding: "18px 0 10px",
            }}
          >
            Customer rating
          </div>
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              gap: 9,
              fontSize: 13,
              paddingBottom: 18,
              borderBottom: "1px solid var(--plt-border-heavy)",
            }}
          >
            <div style={{ color: "var(--plt-star)", letterSpacing: ".5px" }}>
              ★★★★☆{" "}
              <span style={{ color: "var(--plt-terracotta)" }}>&amp; up</span>
            </div>
            <div style={{ color: "var(--plt-star)", letterSpacing: ".5px" }}>
              ★★★☆☆{" "}
              <span style={{ color: "var(--plt-terracotta)" }}>&amp; up</span>
            </div>
          </div>

          {/* Farm filter */}
          <div
            style={{
              fontSize: 14,
              fontWeight: 800,
              padding: "18px 0 10px",
            }}
          >
            Farm / Co-op
          </div>
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              gap: 8,
              fontSize: 13,
            }}
          >
            {["Vom Farms", "Bukuru Co-op", "Riyom Growers", "Barkin Ladi Farms", "Gyero Gardens", "Heipang Produce"].map(
              (farm) => (
                <label
                  key={farm}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 9,
                    cursor: "pointer",
                  }}
                >
                  <span
                    style={{
                      width: 15,
                      height: 15,
                      border: "1.5px solid var(--plt-text-muted)",
                      borderRadius: 3,
                      display: "inline-block",
                    }}
                  />
                  {farm}
                </label>
              )
            )}
          </div>
        </div>

        {/* ── Main grid ────────────────────────────────────────────────── */}
        <div style={{ flex: 1, minWidth: 280 }}>
          {/* Toolbar */}
          <div
            style={{
              background: "var(--plt-card)",
              borderRadius: "var(--plt-radius-sm)",
              padding: "11px 16px",
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              marginBottom: 16,
              flexWrap: "wrap",
              gap: 10,
            }}
          >
            <div style={{ fontSize: 13, color: "var(--plt-text-secondary)" }}>
              Showing{" "}
              <b style={{ color: "var(--plt-text)" }}>{products.length}</b>{" "}
              products
              {q && (
                <>
                  {" "}
                  for <b style={{ color: "var(--plt-text)" }}>&quot;{q}&quot;</b>
                </>
              )}
            </div>
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: 10,
                fontSize: 13,
              }}
            >
              Sort by
              <form action="/products" style={{ display: "inline" }}>
                {q && <input type="hidden" name="q" value={q} />}
                <SortSelect
                  options={SORT_OPTIONS}
                  value={sort}
                  style={{
                    border: "1px solid var(--plt-border-mid)",
                    borderRadius: "var(--plt-radius-sm)",
                    padding: "7px 10px",
                    fontSize: 13,
                    fontWeight: 600,
                    background: "var(--plt-card)",
                    cursor: "pointer",
                  }}
                />
              </form>
            </div>
          </div>

          {/* Empty state */}
          {products.length === 0 && (
            <div
              style={{
                background: "var(--plt-card)",
                borderRadius: "var(--plt-radius-md)",
                padding: "60px 20px",
                textAlign: "center",
                color: "var(--plt-text-secondary)",
              }}
            >
              <div
                style={{
                  fontSize: 16,
                  fontWeight: 700,
                  color: "var(--plt-text)",
                  marginBottom: 6,
                }}
              >
                No produce found
              </div>
              Try another search or{" "}
              <Link
                href="/products"
                style={{ color: "var(--plt-terracotta)", fontWeight: 600 }}
              >
                clear your search
              </Link>
              .
            </div>
          )}

          {/* Product grid */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(190px, 1fr))",
              gap: 16,
            }}
          >
            {products.map((p) => (
              <Link
                key={p.id}
                href={`/products/${p.id}`}
                style={{
                  background: "var(--plt-card)",
                  borderRadius: "var(--plt-radius-md)",
                  padding: 14,
                  display: "flex",
                  flexDirection: "column",
                  textDecoration: "none",
                  color: "var(--plt-text)",
                  transition: "box-shadow 0.2s",
                }}
              >
                <div
                  className="plt-img-placeholder"
                  style={
                    p.image_url
                      ? {
                          backgroundImage: `url("${p.image_url}")`,
                          backgroundSize: "cover",
                          backgroundPosition: "center",
                        }
                      : undefined
                  }
                >
                  {p.available > 0 && (
                    <span className="plt-badge plt-badge-dark">In stock</span>
                  )}
                  {p.available <= 0 && (
                    <span className="plt-badge plt-badge-red">Sold out</span>
                  )}
                  {!p.image_url && (
                    <span
                      style={{
                        fontFamily: "monospace",
                        fontSize: 10,
                        color: "var(--plt-text-muted)",
                      }}
                    >
                      {p.sku}
                    </span>
                  )}
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
                <div className="plt-stars">
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

          {/* Pagination */}
          <nav
            style={{
              marginTop: 24,
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              background: "var(--plt-card)",
              borderRadius: "var(--plt-radius-md)",
              padding: "12px 16px",
              fontSize: 13,
            }}
          >
            <PageLink q={q} sort={sort} page={page - 1} disabled={page <= 1}>
              ← Prev
            </PageLink>
            <span style={{ color: "var(--plt-text-secondary)" }}>
              Page {page} of {totalPages} · {total} items
            </span>
            <PageLink q={q} sort={sort} page={page + 1} disabled={page >= totalPages}>
              Next →
            </PageLink>
          </nav>
        </div>
      </div>
    </main>
  );
}

function PageLink({
  q,
  sort,
  page,
  disabled,
  children,
}: {
  q: string;
  sort: string;
  page: number;
  disabled: boolean;
  children: React.ReactNode;
}) {
  if (disabled)
    return (
      <span style={{ color: "var(--plt-text-muted)" }}>{children}</span>
    );
  const qs = new URLSearchParams();
  if (q) qs.set("q", q);
  // Keep the active sort across pages; "featured" is the default, so omit it.
  if (sort && sort !== "featured") qs.set("sort", sort);
  qs.set("page", String(page));
  return (
    <Link
      href={`/products?${qs}`}
      style={{
        fontWeight: 600,
        color: "var(--plt-terracotta)",
        textDecoration: "none",
      }}
    >
      {children}
    </Link>
  );
}
