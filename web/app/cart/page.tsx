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
  const cartCount = lines.reduce((sum, l) => sum + l.item.quantity, 0);

  // Delivery logic matching the mockup
  const FREE_DELIVERY_OVER = 5000000; // 50,000 NGN in kobo
  const STANDARD_FEE = 350000; // 3,500 NGN
  const subtotal = total;
  const freeDelivery = subtotal >= FREE_DELIVERY_OVER || subtotal === 0;
  const fee = freeDelivery ? 0 : STANDARD_FEE;
  const grandTotal = subtotal + fee;
  const needMore = FREE_DELIVERY_OVER - subtotal;

  return (
    <main style={{ maxWidth: 1080, margin: "0 auto", padding: "24px 20px 60px" }}>
      <h1 style={{ fontSize: 26, fontWeight: 800, marginBottom: 18, margin: 0, paddingBottom: 18 }}>
        Your cart
      </h1>

      {/* ── Empty State ───────────────────────────────────────────────────── */}
      {lines.length === 0 && (
        <div
          className="plt-card-lg"
          style={{ padding: "60px 20px", textAlign: "center" }}
        >
          <div style={{ fontSize: 18, fontWeight: 700, marginBottom: 6 }}>
            Your cart is empty
          </div>
          <div
            style={{
              fontSize: 14,
              color: "var(--plt-text-secondary)",
              marginBottom: 20,
            }}
          >
            Browse this week&apos;s harvest from the Plateau.
          </div>
          <Link href="/products" className="plt-btn-primary-lg">
            Browse produce
          </Link>
        </div>
      )}

      {/* ── Cart with items ───────────────────────────────────────────────── */}
      {lines.length > 0 && (
        <div
          style={{
            display: "flex",
            gap: 20,
            alignItems: "flex-start",
            flexWrap: "wrap",
          }}
        >
          {/* Items list */}
          <div
            className="plt-card-lg"
            style={{ flex: 1, minWidth: 300, padding: "8px 22px" }}
          >
            {lines.map(({ item, product }) => (
              <div
                key={item.product_id}
                style={{
                  display: "flex",
                  gap: 16,
                  padding: "18px 0",
                  borderBottom: "1px solid var(--plt-border)",
                  alignItems: "center",
                  flexWrap: "wrap",
                }}
              >
                {/* Product image placeholder */}
                <div
                  style={{
                    width: 84,
                    height: 84,
                    flex: "0 0 84px",
                    background:
                      "repeating-linear-gradient(45deg, #eceff3 0 7px, #f5f7f9 7px 14px)",
                    borderRadius: "var(--plt-radius-md)",
                    display: "flex",
                    alignItems: "flex-end",
                    padding: 7,
                  }}
                >
                  <span
                    style={{
                      fontFamily: "monospace",
                      fontSize: 9,
                      color: "var(--plt-text-muted)",
                    }}
                  >
                    {product?.sku ?? "N/A"}
                  </span>
                </div>

                {/* Product info */}
                <div style={{ flex: 1, minWidth: 160 }}>
                  <div style={{ fontSize: 15, fontWeight: 700 }}>
                    {product ? product.name : "Unavailable product"}
                  </div>
                  <div
                    style={{
                      fontSize: 12,
                      color: "var(--plt-text-secondary)",
                      margin: "3px 0",
                    }}
                  >
                    {product
                      ? formatPrice(product.price_cents, product.currency)
                      : "—"}{" "}
                    × {item.quantity}
                  </div>
                  <div
                    style={{
                      fontSize: 12,
                      color: "var(--plt-green-text)",
                      fontWeight: 700,
                    }}
                  >
                    {product && product.available > 0 ? "In stock" : ""}
                  </div>
                </div>

                {/* Quantity stepper */}
                <div className="plt-qty-stepper">
                  <form action={updateCartItemAction}>
                    <input type="hidden" name="product_id" value={item.product_id} />
                    <input
                      type="hidden"
                      name="quantity"
                      value={Math.max(0, item.quantity - 1)}
                    />
                    <button type="submit" className="plt-qty-btn" style={{ width: 34, height: 34, fontSize: 17 }}>
                      −
                    </button>
                  </form>
                  <span className="plt-qty-val" style={{ width: 38, fontSize: 14 }}>
                    {item.quantity}
                  </span>
                  <form action={updateCartItemAction}>
                    <input type="hidden" name="product_id" value={item.product_id} />
                    <input
                      type="hidden"
                      name="quantity"
                      value={item.quantity + 1}
                    />
                    <button type="submit" className="plt-qty-btn" style={{ width: 34, height: 34, fontSize: 17 }}>
                      +
                    </button>
                  </form>
                </div>

                {/* Line total */}
                <div
                  style={{
                    width: 110,
                    textAlign: "right",
                    fontSize: 16,
                    fontWeight: 800,
                  }}
                >
                  {product
                    ? formatPrice(
                        product.price_cents * item.quantity,
                        product.currency,
                      )
                    : "—"}
                </div>

                {/* Remove */}
                <form action={removeCartItemAction}>
                  <input type="hidden" name="product_id" value={item.product_id} />
                  <button
                    style={{
                      border: 0,
                      background: "none",
                      color: "var(--plt-terracotta)",
                      fontSize: 13,
                      fontWeight: 600,
                      cursor: "pointer",
                    }}
                  >
                    Remove
                  </button>
                </form>
              </div>
            ))}
          </div>

          {/* Order summary */}
          <div
            className="plt-card-lg"
            style={{ width: 320, flex: "0 0 320px" }}
          >
            <div style={{ fontSize: 17, fontWeight: 800, marginBottom: 16 }}>
              Order summary
            </div>
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                fontSize: 14,
                marginBottom: 10,
              }}
            >
              <span style={{ color: "var(--plt-text-secondary)" }}>
                Subtotal ({cartCount} items)
              </span>
              <b>{formatPrice(subtotal, "NGN")}</b>
            </div>
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                fontSize: 14,
                marginBottom: 10,
              }}
            >
              <span style={{ color: "var(--plt-text-secondary)" }}>
                Delivery
              </span>
              <b>{freeDelivery ? "Free" : formatPrice(fee, "NGN")}</b>
            </div>

            {/* Free delivery hint */}
            {subtotal > 0 && needMore > 0 && (
              <div
                style={{
                  fontSize: 12,
                  color: "var(--plt-green-text)",
                  background: "var(--plt-green-bg-light)",
                  padding: "8px 10px",
                  borderRadius: "var(--plt-radius-sm)",
                  marginBottom: 10,
                }}
              >
                Add {formatPrice(needMore, "NGN")} more for free delivery
              </div>
            )}

            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                fontSize: 18,
                fontWeight: 800,
                borderTop: "1px solid var(--plt-border-heavy)",
                paddingTop: 14,
                marginTop: 4,
              }}
            >
              <span>Total</span>
              <span>{formatPrice(grandTotal, "NGN")}</span>
            </div>

            <Link
              href="/checkout"
              className="plt-btn-gold"
              style={{
                display: "block",
                textAlign: "center",
                textDecoration: "none",
                marginTop: 18,
              }}
            >
              Proceed to checkout
            </Link>
            <Link
              href="/products"
              className="plt-btn-outline"
              style={{
                display: "block",
                textAlign: "center",
                textDecoration: "none",
                marginTop: 10,
              }}
            >
              Continue shopping
            </Link>
          </div>
        </div>
      )}
    </main>
  );
}
