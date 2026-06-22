import Link from "next/link";
import { notFound } from "next/navigation";
import { getProduct, GatewayError } from "@/lib/gateway";
import { formatPrice } from "@/lib/format";
import { addToCartAction } from "@/app/cart/actions";

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
    product = await getProduct(id);
  } catch (err) {
    if (err instanceof GatewayError && err.status === 404) notFound();
    throw err;
  }

  return (
    <main className="mx-auto max-w-3xl px-6 py-10">
      <Link href="/products" className="text-sm text-zinc-500 underline">
        ← Back to products
      </Link>
      <h1 className="mt-4 text-3xl font-semibold">{product.name}</h1>
      <p className="mt-2 text-xl">{formatPrice(product.price_cents, product.currency)}</p>
      <p className="mt-1 text-sm text-zinc-500">SKU {product.sku}</p>
      <p className="mt-1 text-sm text-zinc-500">
        {product.available > 0 ? `${product.available} in stock` : "Out of stock"}
      </p>
      {product.description && <p className="mt-6 text-zinc-700">{product.description}</p>}

      {/* Add-to-cart is a plain form bound to a Server Action. A logged-out user's
          request 401s at the gateway and the action redirects them to /login. */}
      <form action={addToCartAction} className="mt-8">
        <input type="hidden" name="product_id" value={product.id} />
        <input type="hidden" name="quantity" value="1" />
        <button
          disabled={product.available <= 0}
          className="rounded-md bg-foreground px-4 py-2 font-medium text-background disabled:opacity-50"
        >
          {product.available > 0 ? "Add to cart" : "Out of stock"}
        </button>
      </form>
    </main>
  );
}
