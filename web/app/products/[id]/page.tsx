import Link from "next/link";
import { notFound } from "next/navigation";
import { getProduct, GatewayError } from "@/lib/gateway";
import { formatPrice } from "@/lib/format";

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
    </main>
  );
}
