import Link from "next/link";
import { notFound, redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { getOrder } from "@/lib/orders";
import { formatPrice } from "@/lib/format";
import { StatusPoller } from "./status-poller";

// The checkout result view. After place-order redirects here (or the PSP returns
// the customer here post-payment), this shows the saga outcome — and while the
// order is still PAYMENT_PENDING it polls until the webhook settles it.
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

  const isCancelled = order.status === "CANCELLED";
  // CONFIRMED/SHIPPED/DELIVERED are all successful, post-payment states.
  const isSuccess = order.status === "CONFIRMED" || order.status === "SHIPPED" || order.status === "DELIVERED";
  const isPending = !isCancelled && !isSuccess; // PAYMENT_PENDING and pre-payment
  const heading =
    { CONFIRMED: "Order confirmed", SHIPPED: "Order shipped", DELIVERED: "Order delivered", CANCELLED: "Order cancelled" }[
      order.status
    ] ?? "Awaiting payment";

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
            background: isSuccess
              ? "var(--plt-green-bg-light)"
              : isCancelled
                ? "var(--plt-error-bg)"
                : "var(--plt-surface)",
            color: isSuccess
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
          {order.status === "DELIVERED" ? "📦" : order.status === "SHIPPED" ? "🚚" : isSuccess ? "✓" : isCancelled ? "✕" : "⏳"}
        </div>

        {/* While the order awaits its payment outcome, poll until it settles. */}
        {isPending && <StatusPoller />}

        <h1 style={{ fontSize: 24, fontWeight: 800, marginBottom: 8, marginTop: 0 }}>{heading}</h1>

        <div
          style={{
            fontSize: 15,
            color: "var(--plt-text-secondary)",
            marginBottom: 22,
          }}
        >
          {order.status === "CONFIRMED" && <>Thank you. We&apos;re preparing your order.</>}
          {order.status === "SHIPPED" && (
            <>
              Your order is on its way
              {order.tracking_number ? (
                <>
                  {" "}
                  · tracking <b>{order.tracking_number}</b>
                </>
              ) : null}
              .
            </>
          )}
          {order.status === "DELIVERED" && <>Delivered. Enjoy your harvest!</>}
          {isCancelled && (
            <>
              Your order was cancelled (payment declined). You were not charged
              and the reserved stock was released.
            </>
          )}
          {isPending && (
            <>
              We&apos;re confirming your payment. This page will update
              automatically — no need to refresh.
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

          {order.shipping_method_name && (
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                marginBottom: 10,
              }}
            >
              <span style={{ color: "var(--plt-text-secondary)" }}>
                Delivery — {order.shipping_method_name}
              </span>
              <b>{order.shipping_cents === 0 ? "Free" : formatPrice(order.shipping_cents, order.currency)}</b>
            </div>
          )}

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

        {order.shipping_address && (
          <div className="plt-card-lg" style={{ marginTop: 16 }}>
            <div style={{ fontSize: 14, fontWeight: 800, marginBottom: 10 }}>Delivery address</div>
            <div style={{ fontSize: 14, lineHeight: 1.5, color: "var(--plt-text-secondary)" }}>
              <div style={{ fontWeight: 700, color: "var(--plt-text)" }}>{order.shipping_address.recipient}</div>
              <div>
                {order.shipping_address.line1}
                {order.shipping_address.line2 ? `, ${order.shipping_address.line2}` : ""}, {order.shipping_address.city},{" "}
                {order.shipping_address.state}
                {order.shipping_address.postal_code ? ` ${order.shipping_address.postal_code}` : ""},{" "}
                {order.shipping_address.country}
              </div>
              <div>{order.shipping_address.phone}</div>
            </div>
          </div>
        )}

        {order.status === "CONFIRMED" && (
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
