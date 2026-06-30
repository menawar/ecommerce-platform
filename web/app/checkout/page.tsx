import Link from "next/link";
import { redirect } from "next/navigation";

import { getCart } from "@/lib/cart";
import { getProduct, GatewayError } from "@/lib/gateway";
import { listAddresses } from "@/lib/addresses";
import { listShippingMethods } from "@/lib/shipping";
import { formatPrice } from "@/lib/format";
import type { Product } from "@/lib/types";
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
    <main style={{ maxWidth: 1080, margin: "0 auto", padding: "24px 20px 60px" }}>
      <Link
        href="/cart"
        style={{
          fontSize: 13,
          color: "var(--plt-terracotta)",
          fontWeight: 600,
          textDecoration: "none",
          display: "inline-block",
          marginBottom: 12,
        }}
      >
        ‹ Back to cart
      </Link>
      <h1 style={{ fontSize: 26, fontWeight: 800, marginBottom: 18, marginTop: 0 }}>Checkout</h1>

      {lines.length === 0 ? (
        <div className="plt-card-lg" style={{ padding: "60px 20px", textAlign: "center" }}>
          <div style={{ fontSize: 16, fontWeight: 700, marginBottom: 6 }}>Your cart is empty</div>
          <p style={{ fontSize: 14, color: "var(--plt-text-secondary)" }}>
            <Link href="/products" style={{ color: "var(--plt-terracotta)" }}>
              Browse products
            </Link>{" "}
            to add items to your cart.
          </p>
        </div>
      ) : (
        <>
          {/* Items preview */}
          <div className="plt-card-lg" style={{ marginBottom: 18 }}>
            <div style={{ fontSize: 16, fontWeight: 800, marginBottom: 14 }}>Items in your order</div>
            {lines.map(({ item, product }) => (
              <div
                key={item.product_id}
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  padding: "8px 0",
                  fontSize: 14,
                  borderBottom: "1px solid var(--plt-border)",
                }}
              >
                <span>
                  {product ? product.name : "Unavailable product"} × {item.quantity}
                </span>
                <span style={{ fontWeight: 600 }}>
                  {product ? formatPrice(product.price_cents * item.quantity, product.currency) : "—"}
                </span>
              </div>
            ))}
            <p style={{ marginTop: 12, fontSize: 12, color: "var(--plt-text-muted)" }}>
              Mock payment: a total whose kobo ends in 13 (e.g. ₦13.13) is declined — use it to see the
              saga&apos;s failure path (order cancelled, stock released).
            </p>
          </div>

          <CheckoutForm
            subtotalCents={subtotal}
            currency="NGN"
            addresses={addresses}
            shippingMethods={shippingMethods}
          />
        </>
      )}
    </main>
  );
}
