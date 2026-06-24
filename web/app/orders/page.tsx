import Link from "next/link";
import { redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { listOrders } from "@/lib/orders";
import { formatPrice, formatDate } from "@/lib/format";

const statusStyle: Record<string, string> = {
  CONFIRMED: "bg-green-50 text-green-700",
  CANCELLED: "bg-red-50 text-red-700",
};

// Order history. A Server Component: it calls the gateway with the user's cookie
// on the server, so the order list (and the JWT) never touch the browser. Status
// here is already terminal — our saga resolves CONFIRMED/CANCELLED synchronously
// before the order is fetchable, so there's nothing to poll for.
export default async function OrdersPage() {
  let orders;
  try {
    orders = await listOrders();
  } catch (err) {
    // A missing/expired cookie reads as 401 at the gateway — bounce to login.
    // Anything else (e.g. gateway down) bubbles to the route's error boundary.
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err;
  }

  return (
    <main className="mx-auto max-w-2xl px-6 py-10">
      <h1 className="text-2xl font-semibold">Your orders</h1>

      {orders.length === 0 ? (
        <p className="mt-6 text-zinc-600">
          You haven&apos;t placed any orders yet.{" "}
          <Link href="/products" className="font-medium underline">
            Browse products
          </Link>
          .
        </p>
      ) : (
        <ul className="mt-6 divide-y divide-zinc-200">
          {orders.map((o) => (
            <li key={o.id}>
              <Link
                href={`/orders/${o.id}`}
                className="flex items-center justify-between py-4 hover:bg-zinc-50"
              >
                <div>
                  <span
                    className={`inline-block rounded px-2 py-0.5 text-xs font-medium ${statusStyle[o.status] ?? "bg-zinc-100 text-zinc-700"}`}
                  >
                    {o.status}
                  </span>
                  <p className="mt-1 text-sm text-zinc-500">{formatDate(o.created_at)}</p>
                </div>
                <span className="font-medium">{formatPrice(o.total_cents, o.currency)}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
