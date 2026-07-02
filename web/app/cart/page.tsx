import Link from "next/link";
import { redirect } from "next/navigation";

import { getCart } from "@/lib/cart";
import { getProduct, GatewayError } from "@/lib/gateway";
import { formatPrice } from "@/lib/format";
import type { Product } from "@/lib/types";
import { Container } from "@/components/ui/container";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { buttonVariants } from "@/components/ui/button";
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

  const subtotal = lines.reduce(
    (sum, l) => sum + (l.product ? l.product.price_cents * l.item.quantity : 0),
    0,
  );
  const cartCount = lines.reduce((sum, l) => sum + l.item.quantity, 0);

  // Delivery logic matching the mockup.
  const FREE_DELIVERY_OVER = 5000000; // ₦50,000 in kobo
  const STANDARD_FEE = 350000; // ₦3,500
  const freeDelivery = subtotal >= FREE_DELIVERY_OVER || subtotal === 0;
  const fee = freeDelivery ? 0 : STANDARD_FEE;
  const grandTotal = subtotal + fee;
  const needMore = FREE_DELIVERY_OVER - subtotal;

  return (
    <Container as="main" size="lg" className="pb-14 pt-6">
      <h1 className="mb-5 text-2xl font-extrabold">Your cart</h1>

      {lines.length === 0 ? (
        <Card padded={false}>
          <EmptyState
            as="h2"
            icon="🧺"
            title="Your cart is empty"
            description="Browse this week's harvest from the Plateau."
            action={
              <Link href="/products" className={buttonVariants({ size: "lg" })}>
                Browse produce
              </Link>
            }
          />
        </Card>
      ) : (
        <div className="flex flex-col gap-5 lg:flex-row lg:items-start">
          {/* Items */}
          <Card className="min-w-0 flex-1 py-1">
            {lines.map(({ item, product }) => (
              <div
                key={item.product_id}
                className="flex flex-wrap items-center gap-4 border-b border-border py-4 last:border-b-0"
              >
                <div className="flex h-20 w-20 flex-none items-end rounded-md bg-surface p-1.5">
                  <span className="font-mono text-[9px] text-fg-subtle">{product?.sku ?? "N/A"}</span>
                </div>

                <div className="min-w-[160px] flex-1">
                  <div className="text-[15px] font-bold">{product ? product.name : "Unavailable product"}</div>
                  <div className="my-0.5 text-xs text-fg-muted">
                    {product ? formatPrice(product.price_cents, product.currency) : "—"} × {item.quantity}
                  </div>
                  {product && product.available > 0 && (
                    <div className="text-xs font-bold text-brand">In stock</div>
                  )}
                </div>

                {/* Quantity stepper (each button is a server-action form submit). */}
                <div className="flex items-center overflow-hidden rounded-md border border-border-strong">
                  <form action={updateCartItemAction}>
                    <input type="hidden" name="product_id" value={item.product_id} />
                    <input type="hidden" name="quantity" value={Math.max(0, item.quantity - 1)} />
                    <button
                      type="submit"
                      aria-label="Decrease quantity"
                      className="flex h-9 w-9 items-center justify-center text-lg hover:bg-surface"
                    >
                      −
                    </button>
                  </form>
                  <span className="w-9 text-center text-sm font-semibold" aria-live="polite">
                    {item.quantity}
                  </span>
                  <form action={updateCartItemAction}>
                    <input type="hidden" name="product_id" value={item.product_id} />
                    <input type="hidden" name="quantity" value={item.quantity + 1} />
                    <button
                      type="submit"
                      aria-label="Increase quantity"
                      className="flex h-9 w-9 items-center justify-center text-lg hover:bg-surface"
                    >
                      +
                    </button>
                  </form>
                </div>

                <div className="w-[110px] text-right text-base font-extrabold">
                  {product ? formatPrice(product.price_cents * item.quantity, product.currency) : "—"}
                </div>

                <form action={removeCartItemAction}>
                  <input type="hidden" name="product_id" value={item.product_id} />
                  <button className="text-sm font-semibold text-accent hover:underline">Remove</button>
                </form>
              </div>
            ))}
          </Card>

          {/* Order summary */}
          <Card as="aside" className="w-full lg:w-[320px] lg:flex-none">
            <div className="mb-4 text-[17px] font-extrabold">Order summary</div>
            <div className="mb-2.5 flex justify-between text-sm">
              <span className="text-fg-muted">Subtotal ({cartCount} items)</span>
              <b>{formatPrice(subtotal, "NGN")}</b>
            </div>
            <div className="mb-2.5 flex justify-between text-sm">
              <span className="text-fg-muted">Delivery</span>
              <b>{freeDelivery ? "Free" : formatPrice(fee, "NGN")}</b>
            </div>

            {subtotal > 0 && needMore > 0 && (
              <div className="mb-2.5 rounded-sm bg-brand-subtle px-2.5 py-2 text-xs font-semibold text-brand">
                Add {formatPrice(needMore, "NGN")} more for free delivery
              </div>
            )}

            <div className="mt-1 flex justify-between border-t border-border-strong pt-3.5 text-lg font-extrabold">
              <span>Total</span>
              <span>{formatPrice(grandTotal, "NGN")}</span>
            </div>

            <Link href="/checkout" className={buttonVariants({ variant: "gold", size: "lg" }) + " mt-4 w-full"}>
              Proceed to checkout
            </Link>
            <Link href="/products" className={buttonVariants({ variant: "outline", size: "lg" }) + " mt-2.5 w-full"}>
              Continue shopping
            </Link>
          </Card>
        </div>
      )}
    </Container>
  );
}
