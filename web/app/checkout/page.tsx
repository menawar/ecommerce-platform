import Link from "next/link";
import { redirect } from "next/navigation";

import { getCart } from "@/lib/cart";
import { getProduct, GatewayError } from "@/lib/gateway";
import { formatPrice } from "@/lib/format";
import type { Product } from "@/lib/types";
import { CheckoutButton } from "./checkout-button";

export default async function CheckoutPage() {
  let cart;
  try {
    cart = await getCart();
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err;
  }

  // Re-resolve prices from the Product service (the cart never stores them). This
  // is a preview; the saga independently re-prices server-side at order time.
  const lines = await Promise.all(
    cart.items.map(async (item) => {
      let product: Product | null = null;
      try {
        product = await getProduct(item.product_id);
      } catch {
        product = null;
      }
      return { item, product };
    }),
  );
  const total = lines.reduce((sum, l) => sum + (l.product ? l.product.price_cents * l.item.quantity : 0), 0);

  return (
    <main className="mx-auto max-w-2xl px-6 py-10">
      <h1 className="text-2xl font-semibold">Checkout</h1>

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
              <li key={item.product_id} className="flex justify-between py-3 text-sm">
                <span>
                  {product ? product.name : "Unavailable product"} × {item.quantity}
                </span>
                <span>
                  {product ? formatPrice(product.price_cents * item.quantity, product.currency) : "—"}
                </span>
              </li>
            ))}
          </ul>
          <div className="mt-4 flex justify-between border-t border-zinc-200 pt-4 font-semibold">
            <span>Total</span>
            <span>{formatPrice(total, "NGN")}</span>
          </div>
          <p className="mt-4 text-xs text-zinc-500">
            Mock payment: a total where the kobo ends in 13 (e.g. ₦13.13) is declined — use it to
            see the saga&apos;s failure path (order cancelled, stock released).
          </p>
          <CheckoutButton />
        </>
      )}
    </main>
  );
}
