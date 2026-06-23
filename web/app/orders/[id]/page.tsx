import Link from "next/link";
import { notFound, redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { getOrder } from "@/lib/orders";
import { formatPrice } from "@/lib/format";

const statusStyle: Record<string, string> = {
  CONFIRMED: "bg-green-50 text-green-700",
  CANCELLED: "bg-red-50 text-red-700",
};

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

  return (
    <main className="mx-auto max-w-2xl px-6 py-10">
      <h1 className="text-2xl font-semibold">Order</h1>
      <span
        className={`mt-3 inline-block rounded px-2 py-1 text-sm font-medium ${statusStyle[order.status] ?? "bg-zinc-100 text-zinc-700"}`}
      >
        {order.status}
      </span>

      {order.status === "CONFIRMED" && (
        <p className="mt-3 text-zinc-600">Thank you — your order is confirmed and stock is committed.</p>
      )}
      {order.status === "CANCELLED" && (
        <p className="mt-3 text-zinc-600">
          Your order was cancelled (payment declined). You were not charged and the reserved stock
          was released.
        </p>
      )}

      <ul className="mt-6 divide-y divide-zinc-200">
        {order.items.map((it) => (
          <li key={it.product_id} className="flex justify-between py-3 text-sm">
            <span>
              {it.name} × {it.quantity}
            </span>
            <span>{formatPrice(it.price_cents * it.quantity, order.currency)}</span>
          </li>
        ))}
      </ul>
      <div className="mt-4 flex justify-between border-t border-zinc-200 pt-4 font-semibold">
        <span>Total</span>
        <span>{formatPrice(order.total_cents, order.currency)}</span>
      </div>

      <Link href="/products" className="mt-8 inline-block font-medium underline">
        Continue shopping
      </Link>
    </main>
  );
}
