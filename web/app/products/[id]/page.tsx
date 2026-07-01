import { cache } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { getProduct, listProducts, GatewayError } from "@/lib/gateway";
import { formatPrice } from "@/lib/format";
import { SITE_URL } from "@/lib/site";
import { addToCartAction } from "@/app/cart/actions";

// Request-scoped memoization: generateMetadata and the page body both need the
// product, but gatewayFetch is cache:no-store, so without this they'd be two
// round-trips per view. cache() collapses them to one within a single render.
const loadProduct = cache((id: string) => getProduct(id));

// Per-product SEO: a real title/description + OpenGraph image so shared links and
// search results show the product, not the generic site card.
export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await params;
  try {
    const p = await loadProduct(id);
    const description = p.description || `${p.name} — fresh from the Jos Plateau.`;
    return {
      title: p.name,
      description,
      alternates: { canonical: `/products/${p.id}` },
      openGraph: {
        title: p.name,
        description,
        type: "website",
        url: `${SITE_URL}/products/${p.id}`,
        images: p.image_url ? [{ url: p.image_url, alt: p.name }] : undefined,
      },
    };
  } catch {
    return { title: "Product" };
  }
}

// params is async in Next 16. We translate a gateway 404 into Next's notFound()
// (renders the nearest not-found UI); any other failure rethrows to error.tsx.
export default async function ProductDetail({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  let product;
  try {
    product = await loadProduct(id);
  } catch (err) {
    if (err instanceof GatewayError && err.status === 404) notFound();
    throw err;
  }

  // Fetch related products
  let related: Awaited<ReturnType<typeof listProducts>>["products"] = [];
  try {
    const result = await listProducts({ page: 1, pageSize: 4 });
    related = result.products.filter((p) => p.id !== product.id).slice(0, 4);
  } catch {
    // silently skip related products if fetch fails
  }

  // Product structured data (schema.org) so search engines can show a rich result
  // with price + availability. Rendered as a JSON-LD script.
  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "Product",
    name: product.name,
    description: product.description || undefined,
    sku: product.sku,
    image: product.image_url || undefined,
    offers: {
      "@type": "Offer",
      price: (product.price_cents / 100).toFixed(2),
      priceCurrency: product.currency || "NGN",
      availability: product.available > 0 ? "https://schema.org/InStock" : "https://schema.org/OutOfStock",
      url: `${SITE_URL}/products/${product.id}`,
    },
  };

  return (
    <main style={{ maxWidth: 1180, margin: "0 auto", padding: "16px 20px 50px" }}>
      <script
        type="application/ld+json"
        // Escape "<" so a product field containing "</script>" can't break out of
        // this block (XSS / invalid JSON-LD).
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd).replace(/</g, "\\u003c") }}
      />
      {/* Back link */}
      <Link
        href="/products"
        style={{
          fontSize: 13,
          color: "var(--plt-terracotta)",
          fontWeight: 600,
          textDecoration: "none",
          display: "inline-block",
          marginBottom: 16,
        }}
      >
        ‹ Back to results
      </Link>

      <div
        style={{
          display: "flex",
          gap: 30,
          flexWrap: "wrap",
          alignItems: "flex-start",
        }}
      >
        {/* ── Left: Images ─────────────────────────────────────────────── */}
        <div
          style={{
            flex: 1,
            minWidth: 300,
            display: "flex",
            gap: 14,
          }}
        >
          {/* Thumbnails */}
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              gap: 10,
            }}
          >
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="plt-img-thumb" />
            ))}
          </div>

          {/* Main image — falls back to a SKU placeholder when none is set.
              Plain <img> by choice: next/image optimization needs remotePatterns
              config + explicit sizing in this flex layout, deferred to a later
              polish step. Images are served from object storage (MinIO/S3/R2). */}
          {product.image_url ? (
            /* eslint-disable-next-line @next/next/no-img-element */
            <img
              src={product.image_url}
              alt={product.name}
              className="plt-img-placeholder-lg"
              style={{ flex: 1, objectFit: "cover" }}
            />
          ) : (
            <div className="plt-img-placeholder-lg" style={{ flex: 1 }}>
              <span
                style={{
                  fontFamily: "monospace",
                  fontSize: 12,
                  color: "var(--plt-text-muted)",
                }}
              >
                {product.sku}
              </span>
            </div>
          )}
        </div>

        {/* ── Right: Product info ──────────────────────────────────────── */}
        <div style={{ width: 380, flex: "0 0 380px" }}>
          <h1
            style={{
              fontSize: 24,
              fontWeight: 800,
              lineHeight: 1.2,
              margin: 0,
            }}
          >
            {product.name}
          </h1>
          <div
            style={{
              fontSize: 13,
              color: "var(--plt-text-secondary)",
              margin: "7px 0",
            }}
          >
            SKU <b style={{ color: "var(--plt-text)" }}>{product.sku}</b>
          </div>
          <div className="plt-stars" style={{ fontSize: 14 }}>
            ★★★★★{" "}
            <span style={{ color: "var(--plt-terracotta)" }}>
              {product.available} in stock
            </span>
          </div>

          {/* Price section */}
          <div
            style={{
              borderTop: "1px solid var(--plt-border-heavy)",
              margin: "16px 0",
              paddingTop: 16,
              display: "flex",
              alignItems: "baseline",
              gap: 10,
            }}
          >
            <span style={{ fontSize: 32, fontWeight: 800 }}>
              {formatPrice(product.price_cents, product.currency)}
            </span>
          </div>

          <div
            style={{
              fontSize: 14,
              color: "var(--plt-green-text)",
              fontWeight: 700,
              marginBottom: 4,
            }}
          >
            {product.available > 0 ? "In stock · harvested fresh" : "Out of stock"}
          </div>
          <div
            style={{
              fontSize: 13,
              color: "var(--plt-text-secondary)",
              marginBottom: 18,
            }}
          >
            Delivered this week across Jos &amp; Plateau. Free delivery on bulk
            orders over ₦50,000.
          </div>

          {/* Add to cart / Buy now */}
          <div
            style={{
              display: "flex",
              flexDirection: "column",
              gap: 10,
            }}
          >
            {/* Add-to-cart is a plain form bound to a Server Action. A logged-out user's
                request 401s at the gateway and the action redirects them to /login. */}
            <form action={addToCartAction}>
              <input type="hidden" name="product_id" value={product.id} />
              <input type="hidden" name="quantity" value="1" />
              <button
                disabled={product.available <= 0}
                className="plt-btn-primary-lg"
                style={{ width: "100%" }}
              >
                {product.available > 0 ? "Add to cart" : "Out of stock"}
              </button>
            </form>
          </div>

          {/* About this produce */}
          <div
            style={{
              borderTop: "1px solid var(--plt-border-heavy)",
              marginTop: 20,
              paddingTop: 16,
            }}
          >
            <div style={{ fontSize: 14, fontWeight: 800, marginBottom: 8 }}>
              About this product
            </div>
            {product.description && (
              <div
                style={{
                  fontSize: 13,
                  lineHeight: 1.6,
                  color: "#3d444c",
                }}
              >
                {product.description}
              </div>
            )}
            <div
              style={{
                display: "flex",
                flexDirection: "column",
                gap: 6,
                marginTop: 12,
                fontSize: 13,
                color: "#3d444c",
              }}
            >
              <div>✓ Freshly harvested &amp; graded for quality</div>
              <div>✓ SKU {product.sku}</div>
              <div>✓ Cold-chain handled to keep it farm-fresh</div>
            </div>
          </div>
        </div>
      </div>

      {/* ── You might also like ─────────────────────────────────────────── */}
      {related.length > 0 && (
        <div className="plt-card" style={{ marginTop: 26 }}>
          <div style={{ fontSize: 18, fontWeight: 800, marginBottom: 16 }}>
            You might also like
          </div>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(186px, 1fr))",
              gap: 16,
            }}
          >
            {related.map((p) => (
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
                <div className="plt-stars">★★★★★</div>
                <div style={{ fontSize: 16, fontWeight: 800, marginTop: 6 }}>
                  {formatPrice(p.price_cents, p.currency)}
                </div>
              </Link>
            ))}
          </div>
        </div>
      )}
    </main>
  );
}
