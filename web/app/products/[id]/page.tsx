import { cache } from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { getProduct, listProducts, GatewayError } from "@/lib/gateway";
import { formatPrice } from "@/lib/format";
import { SITE_URL } from "@/lib/site";
import { addToCartAction } from "@/app/cart/actions";
import { Container } from "@/components/ui/container";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ProductCard } from "@/components/product-card";

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

  // Fetch related products.
  let related: Awaited<ReturnType<typeof listProducts>>["products"] = [];
  try {
    const result = await listProducts({ page: 1, pageSize: 4 });
    related = result.products.filter((p) => p.id !== product.id).slice(0, 4);
  } catch {
    // silently skip related products if fetch fails
  }

  const inStock = product.available > 0;

  // Product structured data (schema.org) so search engines can show a rich result
  // with price + availability.
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
      availability: inStock ? "https://schema.org/InStock" : "https://schema.org/OutOfStock",
      url: `${SITE_URL}/products/${product.id}`,
    },
  };

  return (
    <Container as="main" className="pb-12 pt-4">
      <script
        type="application/ld+json"
        // Escape "<" so a product field containing "</script>" can't break out.
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd).replace(/</g, "\\u003c") }}
      />

      <Link href="/products" className="mb-4 inline-block text-sm font-semibold text-accent hover:underline">
        ‹ Back to results
      </Link>

      {/* Stacks on mobile; image + buy-box side by side on desktop. */}
      <div className="flex flex-col items-start gap-8 lg:flex-row">
        {/* Images */}
        <div className="flex w-full min-w-0 gap-3 lg:flex-1">
          {/* Thumbnail rail (decorative placeholders until multi-image, Phase D). */}
          <div className="hidden flex-col gap-2.5 sm:flex">
            {[1, 2, 3, 4].map((i) => (
              <div key={i} className="h-14 w-14 rounded-md border border-border bg-surface" />
            ))}
          </div>
          <div className="flex aspect-square flex-1 items-center justify-center overflow-hidden rounded-xl border border-border bg-surface">
            {product.image_url ? (
              /* eslint-disable-next-line @next/next/no-img-element */
              <img src={product.image_url} alt={product.name} className="h-full w-full object-cover" />
            ) : (
              <span className="font-mono text-xs text-fg-subtle">{product.sku}</span>
            )}
          </div>
        </div>

        {/* Buy box */}
        <div className="w-full lg:w-[380px] lg:flex-none">
          <h1 className="text-2xl font-extrabold leading-tight">{product.name}</h1>
          <div className="my-2 text-sm text-fg-muted">
            SKU <b className="text-fg">{product.sku}</b>
          </div>
          <div className="text-sm font-semibold text-accent">
            {inStock ? `${product.available} in stock` : "Out of stock"}
          </div>

          <div className="my-4 flex items-baseline gap-2.5 border-t border-border-strong pt-4">
            <span className="text-[32px] font-extrabold">{formatPrice(product.price_cents, product.currency)}</span>
          </div>

          <div className="mb-1 flex items-center gap-2">
            {inStock ? <Badge variant="brand">In stock</Badge> : <Badge variant="danger">Out of stock</Badge>}
            {inStock && <span className="text-sm font-semibold text-brand">harvested fresh</span>}
          </div>
          <p className="mb-4 text-sm text-fg-muted">
            Delivered this week across Jos &amp; Plateau. Free delivery on bulk orders over ₦50,000.
          </p>

          {/* Add-to-cart is a plain form bound to a Server Action. A logged-out user's
              request 401s at the gateway and the action redirects them to /login. */}
          <form action={addToCartAction}>
            <input type="hidden" name="product_id" value={product.id} />
            <input type="hidden" name="quantity" value="1" />
            <Button type="submit" size="lg" fullWidth disabled={!inStock}>
              {inStock ? "Add to cart" : "Out of stock"}
            </Button>
          </form>

          <div className="mt-5 border-t border-border-strong pt-4">
            <h2 className="mb-2 text-sm font-extrabold">About this product</h2>
            {product.description && (
              <p className="text-sm leading-relaxed text-fg-muted">{product.description}</p>
            )}
            <ul className="mt-3 flex flex-col gap-1.5 text-sm text-fg-muted">
              <li>✓ Freshly harvested &amp; graded for quality</li>
              <li>✓ SKU {product.sku}</li>
              <li>✓ Cold-chain handled to keep it farm-fresh</li>
            </ul>
          </div>
        </div>
      </div>

      {/* You might also like */}
      {related.length > 0 && (
        <Card className="mt-6">
          <h2 className="mb-4 text-lg font-extrabold">You might also like</h2>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
            {related.map((p) => (
              <ProductCard key={p.id} product={p} />
            ))}
          </div>
        </Card>
      )}
    </Container>
  );
}
