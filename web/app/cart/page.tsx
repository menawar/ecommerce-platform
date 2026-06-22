import Link from "next/link";
import { redirect } from "next/navigation";

import { getCart } from "@/lib/cart";
import { getProduct, GatewayError } from "@/lib/gateway";
import { formatPrice } from "@/lib/format";
import type { Product } from "@/lib/types";
import { updateCartItemAction, removeCartItemAction } from "./actions";

export default async function CartPage() {
  let cart;
  try {
    cart = await getCart();
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err;
  }

  // The cart stores ONLY product_id + quantity. Names and prices are resolved here,
  // at display time, from the Product service — never stored in the cart. Fetches
  // run in parallel via Promise.all. (Checkout will likewise re-resolve prices from
  // the authoritative source — the cart's numbers are never trusted for money.)
  const lines = await Promise.all(
    cart.items.map(async (item) => {
      let product: Product | null = null;
      try {
        product = await getProduct(item.product_id);
      } catch {
        product = null; // product since deleted — show a fallback row
      }
      return { item, product };
    }),
  );

  const total = lines.reduce(
    (sum, l) => sum + (l.product ? l.product.price_cents * l.item.quantity : 0),
    0,
  );

  return (
    <main className="mx-auto max-w-3xl px-6 py-10">
      <h1 className="text-2xl font-semibold">Your cart</h1>

      {lines.length === 0 ? (
        <p className="mt-6 text-zinc-600">
          Your cart is empty.{" "}
          <Link href="/products" className="underline">
            Browse products
          </Link>
          .
        </p>
      ) : (
        <>
          <ul className="mt-6 divide-y divide-zinc-200">
            {lines.map(({ item, product }) => (
              <li key={item.product_id} className="flex items-center justify-between gap-4 py-4">
                <div className="min-w-0">
                  <p className="font-medium">{product ? product.name : "Unavailable product"}</p>
                  <p className="text-sm text-zinc-500">
                    {product ? formatPrice(product.price_cents, product.currency) : "—"} ×{" "}
                    {item.quantity} ={" "}
                    {product
                      ? formatPrice(product.price_cents * item.quantity, product.currency)
                      : "—"}
                  </p>
                </div>

                <div className="flex items-center gap-2">
                  {/* Update quantity (0 removes) — a plain form bound to a Server Action. */}
                  <form action={updateCartItemAction} className="flex items-center gap-1">
                    <input type="hidden" name="product_id" value={item.product_id} />
                    <input
                      type="number"
                      name="quantity"
                      defaultValue={item.quantity}
                      min={0}
                      className="w-16 rounded-md border border-zinc-300 px-2 py-1"
                    />
                    <button className="rounded-md border border-zinc-300 px-2 py-1 text-sm">
                      Update
                    </button>
                  </form>
                  <form action={removeCartItemAction}>
                    <input type="hidden" name="product_id" value={item.product_id} />
                    <button className="rounded-md border border-zinc-300 px-2 py-1 text-sm text-red-600">
                      Remove
                    </button>
                  </form>
                </div>
              </li>
            ))}
          </ul>

          <div className="mt-6 flex items-center justify-between border-t border-zinc-200 pt-4">
            <span className="text-lg font-semibold">Total</span>
            <span className="text-lg font-semibold">{formatPrice(total, "NGN")}</span>
          </div>
        </>
      )}
    </main>
  );
}
