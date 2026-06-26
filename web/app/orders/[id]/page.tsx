import Link from "next/link";
import { notFound, redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { getOrder } from "@/lib/orders";
import { formatPrice } from "@/lib/format";

// The checkout result view. After place-order redirects here, this shows whether
// the saga ended CONFIRMED or CANCELLED.
export default async function OrderPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;

  let order;
  try {
    order = await getOrder(id);
  } catch (err) {
    if (err instanceof GatewayError) {
      if (err.status === 401) redirect("/login");
      if (err.status === 404) notFound(); // not yours, or doesn't exist
    }
    throw err;
  }

  const isConfirmed = order.status === "CONFIRMED";
  const isCancelled = order.status === "CANCELLED";

  return (
    <main style={{ maxWidth: 640, margin: "0 auto", padding: "60px 20px" }}>
      <div
        className="plt-card-lg"
        style={{
          borderRadius: "var(--plt-radius-xl)",
          padding: "44px 36px",
          textAlign: "center",
        }}
      >
        {/* Status icon */}
        <div
          style={{
            width: 72,
            height: 72,
            borderRadius: "50%",
            background: isConfirmed
              ? "var(--plt-green-bg-light)"
              : isCancelled
                ? "var(--plt-error-bg)"
                : "var(--plt-surface)",
            color: isConfirmed
              ? "var(--plt-green-text)"
              : isCancelled
                ? "var(--plt-error)"
                : "var(--plt-text-secondary)",
            fontSize: 38,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            margin: "0 auto 20px",
          }}
        >
          {isConfirmed ? "✓" : isCancelled ? "✕" : "?"}
        </div>

        <h1 style={{ fontSize: 24, fontWeight: 800, marginBottom: 8, marginTop: 0 }}>
          {isConfirmed
            ? "Order confirmed"
            : isCancelled
              ? "Order cancelled"
              : `Order ${order.status}`}
        </h1>

        <div
          style={{
            fontSize: 15,
            color: "var(--plt-text-secondary)",
            marginBottom: 22,
          }}
        >
          {isConfirmed && (
            <>Thank you. Your harvest is on the way.</>
          )}
          {isCancelled && (
            <>
              Your order was cancelled (payment declined). You were not charged
              and the reserved stock was released.
            </>
          )}
        </div>

        {/* Order summary */}
        <div
          style={{
            background: "var(--plt-surface-alt)",
            borderRadius: "var(--plt-radius-lg)",
            padding: 20,
            textAlign: "left",
            fontSize: 14,
          }}
        >
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              marginBottom: 10,
            }}
          >
            <span style={{ color: "var(--plt-text-secondary)" }}>
              Order ID
            </span>
            <b style={{ fontFamily: "monospace", fontSize: 12 }}>{order.id}</b>
          </div>

          {order.items.map((it) => (
            <div
              key={it.product_id}
              style={{
                display: "flex",
                justifyContent: "space-between",
                marginBottom: 10,
              }}
            >
              <span style={{ color: "var(--plt-text-secondary)" }}>
                {it.name} × {it.quantity}
              </span>
              <b>{formatPrice(it.price_cents * it.quantity, order.currency)}</b>
            </div>
          ))}

          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              fontSize: 16,
              fontWeight: 800,
              borderTop: "1px solid var(--plt-border-heavy)",
              paddingTop: 12,
              marginTop: 4,
            }}
          >
            <span>Total</span>
            <span>{formatPrice(order.total_cents, order.currency)}</span>
          </div>
        </div>

        {isConfirmed && (
          <div
            style={{
              fontSize: 13,
              color: "var(--plt-green-text)",
              fontWeight: 700,
              marginTop: 18,
            }}
          >
            Estimated delivery: this week, across Jos &amp; Plateau
          </div>
        )}

        <Link
          href="/products"
          className="plt-btn-primary-lg"
          style={{
            display: "inline-block",
            textDecoration: "none",
            marginTop: 24,
          }}
        >
          Continue shopping
        </Link>
      </div>
    </main>
  );
}
