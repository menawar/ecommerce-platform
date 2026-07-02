import Link from "next/link";
import { notFound, redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { getOrder } from "@/lib/orders";
import { formatPrice } from "@/lib/format";
import { cn } from "@/lib/cn";
import { Container } from "@/components/ui/container";
import { Card } from "@/components/ui/card";
import { buttonVariants } from "@/components/ui/button";
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
  const isRefunded = order.status === "REFUNDED";
  // CONFIRMED/SHIPPED/DELIVERED are all successful, post-payment states.
  const isSuccess = order.status === "CONFIRMED" || order.status === "SHIPPED" || order.status === "DELIVERED";
  const isPending = !isCancelled && !isRefunded && !isSuccess; // PAYMENT_PENDING and pre-payment
  const heading =
    {
      CONFIRMED: "Order confirmed",
      SHIPPED: "Order shipped",
      DELIVERED: "Order delivered",
      CANCELLED: "Order cancelled",
      REFUNDED: "Order refunded",
    }[order.status] ?? "Awaiting payment";

  const icon =
    order.status === "DELIVERED"
      ? "📦"
      : order.status === "SHIPPED"
        ? "🚚"
        : isSuccess
          ? "✓"
          : isRefunded
            ? "↩"
            : isCancelled
              ? "✕"
              : "⏳";

  return (
    <Container as="main" size="md" className="pb-14 pt-12">
      <Card padded={false} className="px-6 py-10 text-center sm:px-9">
        <div
          aria-hidden
          className={cn(
            "mx-auto mb-5 flex h-[72px] w-[72px] items-center justify-center rounded-full text-4xl",
            isSuccess ? "bg-brand-subtle text-brand" : isCancelled ? "bg-danger-bg text-danger" : "bg-surface text-fg-muted",
          )}
        >
          {icon}
        </div>

        {/* While the order awaits its payment outcome, poll until it settles. */}
        {isPending && <StatusPoller />}

        <h1 className="mb-2 text-2xl font-extrabold">{heading}</h1>

        <div className="mb-6 text-[15px] text-fg-muted">
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
          {isRefunded && <>This order was refunded — the funds have been returned to your payment method.</>}
          {isCancelled && (
            <>
              Your order was cancelled (payment declined). You were not charged and the reserved stock
              was released.
            </>
          )}
          {isPending && (
            <>We&apos;re confirming your payment. This page will update automatically — no need to refresh.</>
          )}
        </div>

        {/* Order summary */}
        <div className="rounded-lg bg-surface-alt p-5 text-left text-sm">
          <div className="mb-2.5 flex justify-between">
            <span className="text-fg-muted">Order ID</span>
            <b className="font-mono text-xs">{order.id}</b>
          </div>

          {order.items.map((it) => (
            <div key={it.product_id} className="mb-2.5 flex justify-between">
              <span className="text-fg-muted">
                {it.name} × {it.quantity}
              </span>
              <b>{formatPrice(it.price_cents * it.quantity, order.currency)}</b>
            </div>
          ))}

          {order.shipping_method_name && (
            <div className="mb-2.5 flex justify-between">
              <span className="text-fg-muted">Delivery — {order.shipping_method_name}</span>
              <b>{order.shipping_cents === 0 ? "Free" : formatPrice(order.shipping_cents, order.currency)}</b>
            </div>
          )}

          <div className="mt-1 flex justify-between border-t border-border-strong pt-3 text-base font-extrabold">
            <span>Total</span>
            <span>{formatPrice(order.total_cents, order.currency)}</span>
          </div>
        </div>

        {order.shipping_address && (
          <div className="mt-4 rounded-lg border border-border p-4 text-left">
            <div className="mb-2 text-sm font-extrabold">Delivery address</div>
            <div className="text-sm leading-relaxed text-fg-muted">
              <div className="font-bold text-fg">{order.shipping_address.recipient}</div>
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
          <div className="mt-4 text-sm font-bold text-brand">
            Estimated delivery: this week, across Jos &amp; Plateau
          </div>
        )}

        <Link href="/products" className={buttonVariants({ size: "lg" }) + " mt-6"}>
          Continue shopping
        </Link>
      </Card>
    </Container>
  );
}
