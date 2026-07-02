import Link from "next/link";
import type { Product } from "@/lib/types";
import { formatPrice } from "@/lib/format";
import { Badge } from "@/components/ui/badge";

// The shared storefront product tile — used by the catalog grid, the home rows, and
// the "you might also like" strip. One place for the card look, the safe <img>
// (no url() interpolation), and the stock badge. Ratings are intentionally absent
// until Phase E wires real review data (no fabricated stars).
export function ProductCard({ product }: { product: Product }) {
  const inStock = product.available > 0;
  return (
    <Link
      href={`/products/${product.id}`}
      className="group flex flex-col overflow-hidden rounded-xl border border-border bg-card text-fg no-underline shadow-card transition-shadow hover:shadow-md"
    >
      <div className="relative flex aspect-square items-center justify-center overflow-hidden bg-surface">
        <span className="absolute left-2 top-2 z-[1]">
          {inStock ? <Badge variant="brand">In stock</Badge> : <Badge variant="danger">Sold out</Badge>}
        </span>
        {product.image_url ? (
          /* eslint-disable-next-line @next/next/no-img-element */
          <img src={product.image_url} alt={product.name} className="h-full w-full object-cover" />
        ) : (
          <span className="font-mono text-[10px] text-fg-subtle">{product.sku}</span>
        )}
      </div>
      <div className="flex flex-1 flex-col p-3.5">
        <div className="line-clamp-2 min-h-[34px] text-sm leading-snug">{product.name}</div>
        <div className="mt-1.5 text-[17px] font-extrabold">{formatPrice(product.price_cents, product.currency)}</div>
        <div className="mt-0.5 text-[11px] font-bold text-brand">
          {inStock ? "Delivered this week" : "Out of stock"}
        </div>
      </div>
    </Link>
  );
}
