import Link from "next/link";
import { redirect } from "next/navigation";

import { getCart } from "@/lib/cart";
import { getProduct, GatewayError } from "@/lib/gateway";
import { listAddresses } from "@/lib/addresses";
import { listShippingMethods } from "@/lib/shipping";
import { formatPrice } from "@/lib/format";
import type { Product } from "@/lib/types";
import { Container } from "@/components/ui/container";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { buttonVariants } from "@/components/ui/button";
import { CheckoutForm } from "./checkout-form";

export default async function CheckoutPage() {
  let cart, addresses, shippingMethods;
  try {
    [cart, addresses, shippingMethods] = await Promise.all([getCart(), listAddresses(), listShippingMethods()]);
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

  const subtotal = lines.reduce(
    (sum, l) => sum + (l.product ? l.product.price_cents * l.item.quantity : 0),
    0,
  );

  return (
    <Container as="main" size="lg" className="pb-14 pt-6">
      <Link href="/cart" className="mb-3 inline-block text-sm font-semibold text-accent hover:underline">
        ‹ Back to cart
      </Link>
      <h1 className="mb-5 text-2xl font-extrabold">Checkout</h1>

      {lines.length === 0 ? (
        <Card padded={false}>
          <EmptyState
            as="h2"
            icon="🧺"
            title="Your cart is empty"
            description="Add some produce before checking out."
            action={
              <Link href="/products" className={buttonVariants({ size: "lg" })}>
                Browse products
              </Link>
            }
          />
        </Card>
      ) : (
        <>
          {/* Items preview */}
          <Card className="mb-5">
            <h2 className="mb-3.5 text-base font-extrabold">Items in your order</h2>
            {lines.map(({ item, product }) => (
              <div
                key={item.product_id}
                className="flex justify-between border-b border-border py-2 text-sm last:border-b-0"
              >
                <span>
                  {product ? product.name : "Unavailable product"} × {item.quantity}
                </span>
                <span className="font-semibold">
                  {product ? formatPrice(product.price_cents * item.quantity, product.currency) : "—"}
                </span>
              </div>
            ))}
            <p className="mt-3 text-xs text-fg-subtle">
              Mock payment: a total whose kobo ends in 13 (e.g. ₦13.13) is declined — use it to see the
              saga&apos;s failure path (order cancelled, stock released).
            </p>
          </Card>

          <CheckoutForm
            subtotalCents={subtotal}
            currency="NGN"
            addresses={addresses}
            shippingMethods={shippingMethods}
          />
        </>
      )}
    </Container>
  );
}
