import Link from "next/link";
import { redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { listOrders } from "@/lib/orders";
import { formatPrice, formatDate } from "@/lib/format";
import { Container } from "@/components/ui/container";
import { Card } from "@/components/ui/card";
import { Badge, type BadgeVariant } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { buttonVariants } from "@/components/ui/button";

function statusVariant(status: string): BadgeVariant {
  if (status === "CANCELLED") return "danger";
  if (status === "CONFIRMED" || status === "SHIPPED" || status === "DELIVERED") return "success";
  return "neutral"; // REFUNDED, PAYMENT_PENDING, etc.
}

// Order history. A Server Component: it calls the gateway with the user's cookie
// on the server, so the order list (and the JWT) never touch the browser.
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
    <Container as="main" size="lg" className="pb-14 pt-6">
      <h1 className="mb-5 text-2xl font-extrabold">Your orders</h1>

      {orders.length === 0 ? (
        <Card padded={false}>
          <EmptyState
            as="h2"
            icon="📦"
            title="No orders yet"
            description="You haven't placed any orders yet."
            action={
              <Link href="/products" className={buttonVariants({ size: "lg" })}>
                Browse products
              </Link>
            }
          />
        </Card>
      ) : (
        <Card padded={false} className="overflow-hidden">
          {orders.map((o) => (
            <Link
              key={o.id}
              href={`/orders/${o.id}`}
              className="flex items-center justify-between border-b border-border px-5 py-4 text-fg no-underline transition-colors last:border-b-0 hover:bg-surface"
            >
              <div>
                <Badge variant={statusVariant(o.status)}>{o.status}</Badge>
                <div className="mt-1 text-sm text-fg-muted">{formatDate(o.created_at)}</div>
              </div>
              <span className="text-[15px] font-bold">{formatPrice(o.total_cents, o.currency)}</span>
            </Link>
          ))}
        </Card>
      )}
    </Container>
  );
}
