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

  const total = lines.reduce(
    (sum, l) => sum + (l.product ? l.product.price_cents * l.item.quantity : 0),
    0,
  );
  const cartCount = lines.reduce((sum, l) => sum + l.item.quantity, 0);
  const FREE_DELIVERY_OVER = 5000000;
  const STANDARD_FEE = 350000;
  const EXPRESS_FEE = 400000;
  const freeStandard = total >= FREE_DELIVERY_OVER || total === 0;
  const standardFee = freeStandard ? 0 : STANDARD_FEE;
  const grandTotal = total + standardFee;

  return (
    <main style={{ maxWidth: 1080, margin: "0 auto", padding: "24px 20px 60px" }}>
      {/* Back to cart */}
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
      <h1 style={{ fontSize: 26, fontWeight: 800, marginBottom: 18, marginTop: 0 }}>
        Checkout
      </h1>

      {lines.length === 0 ? (
        <div
          className="plt-card-lg"
          style={{ padding: "60px 20px", textAlign: "center" }}
        >
          <div style={{ fontSize: 16, fontWeight: 700, marginBottom: 6 }}>
            Your cart is empty
          </div>
          <p style={{ fontSize: 14, color: "var(--plt-text-secondary)" }}>
            <Link href="/products" style={{ color: "var(--plt-terracotta)" }}>
              Browse products
            </Link>{" "}
            to add items to your cart.
          </p>
        </div>
      ) : (
        <div
          style={{
            display: "flex",
            gap: 20,
            alignItems: "flex-start",
            flexWrap: "wrap",
          }}
        >
          {/* ── Left column ──────────────────────────────────────────────── */}
          <div
            style={{
              flex: 1,
              minWidth: 300,
              display: "flex",
              flexDirection: "column",
              gap: 18,
            }}
          >
            {/* Delivery method */}
            <div className="plt-card-lg">
              <div style={{ fontSize: 16, fontWeight: 800, marginBottom: 14 }}>
                Delivery method
              </div>
              <div
                style={{
                  display: "flex",
                  flexDirection: "column",
                  gap: 10,
                }}
              >
                <div className="plt-radio-card active">
                  <span className="plt-radio-dot">
                    <span className="plt-radio-dot-inner" />
                  </span>
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 14, fontWeight: 700 }}>
                      Standard delivery
                    </div>
                    <div
                      style={{
                        fontSize: 12,
                        color: "var(--plt-text-secondary)",
                      }}
                    >
                      Within Jos &amp; Plateau · 2–3 days
                    </div>
                  </div>
                  <div style={{ fontSize: 14, fontWeight: 800 }}>
                    {freeStandard ? "Free" : formatPrice(standardFee, "NGN")}
                  </div>
                </div>

                <div className="plt-radio-card">
                  <span className="plt-radio-dot" />
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 14, fontWeight: 700 }}>
                      Express delivery
                    </div>
                    <div
                      style={{
                        fontSize: 12,
                        color: "var(--plt-text-secondary)",
                      }}
                    >
                      Next-day in Jos metro
                    </div>
                  </div>
                  <div style={{ fontSize: 14, fontWeight: 800 }}>
                    {formatPrice(EXPRESS_FEE, "NGN")}
                  </div>
                </div>
              </div>
            </div>

            {/* Payment */}
            <div className="plt-card-lg">
              <div style={{ fontSize: 16, fontWeight: 800, marginBottom: 14 }}>
                Payment
              </div>
              <div
                style={{
                  display: "flex",
                  flexDirection: "column",
                  gap: 10,
                }}
              >
                <div className="plt-radio-card active">
                  <span className="plt-radio-dot">
                    <span className="plt-radio-dot-inner" />
                  </span>
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 14, fontWeight: 700 }}>
                      Pay on delivery
                    </div>
                    <div
                      style={{
                        fontSize: 12,
                        color: "var(--plt-text-secondary)",
                      }}
                    >
                      Cash or transfer when it arrives
                    </div>
                  </div>
                </div>

                <div className="plt-radio-card">
                  <span className="plt-radio-dot" />
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 14, fontWeight: 700 }}>
                      Bank transfer
                    </div>
                    <div
                      style={{
                        fontSize: 12,
                        color: "var(--plt-text-secondary)",
                      }}
                    >
                      Pay now via your bank app
                    </div>
                  </div>
                </div>

                <div className="plt-radio-card">
                  <span className="plt-radio-dot" />
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 14, fontWeight: 700 }}>
                      Debit card
                    </div>
                    <div
                      style={{
                        fontSize: 12,
                        color: "var(--plt-text-secondary)",
                      }}
                    >
                      Visa, Verve, Mastercard
                    </div>
                  </div>
                </div>
              </div>
            </div>

            {/* Order items preview */}
            <div className="plt-card-lg">
              <div style={{ fontSize: 16, fontWeight: 800, marginBottom: 14 }}>
                Items in your order
              </div>
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
                    {product ? product.name : "Unavailable product"} ×{" "}
                    {item.quantity}
                  </span>
                  <span style={{ fontWeight: 600 }}>
                    {product
                      ? formatPrice(
                          product.price_cents * item.quantity,
                          product.currency,
                        )
                      : "—"}
                  </span>
                </div>
              ))}
              <p
                style={{
                  marginTop: 12,
                  fontSize: 12,
                  color: "var(--plt-text-muted)",
                }}
              >
                Mock payment: a total where the kobo ends in 13 (e.g. ₦13.13) is
                declined — use it to see the saga&apos;s failure path (order
                cancelled, stock released).
              </p>
            </div>
          </div>

          {/* ── Right: Order summary ─────────────────────────────────────── */}
          <div
            className="plt-card-lg"
            style={{ width: 320, flex: "0 0 320px" }}
          >
            <div style={{ fontSize: 17, fontWeight: 800, marginBottom: 16 }}>
              Your order
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
                Items ({cartCount})
              </span>
              <b>{formatPrice(total, "NGN")}</b>
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
              <b>{freeStandard ? "Free" : formatPrice(standardFee, "NGN")}</b>
            </div>
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

            <CheckoutButton />

            <div
              style={{
                fontSize: 11,
                color: "var(--plt-text-muted)",
                textAlign: "center",
                marginTop: 10,
              }}
            >
              By placing your order you agree to Plateau&apos;s terms.
            </div>
          </div>
        </div>
      )}
    </main>
  );
}
