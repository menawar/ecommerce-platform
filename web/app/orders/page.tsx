import Link from "next/link";
import { redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { listOrders } from "@/lib/orders";
import { formatPrice, formatDate } from "@/lib/format";

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
    <main style={{ maxWidth: 1080, margin: "0 auto", padding: "24px 20px 60px" }}>
      <h1 style={{ fontSize: 26, fontWeight: 800, marginBottom: 18, marginTop: 0 }}>
        Your orders
      </h1>

      {orders.length === 0 ? (
        <div
          className="plt-card-lg"
          style={{ padding: "60px 20px", textAlign: "center" }}
        >
          <div style={{ fontSize: 16, fontWeight: 700, marginBottom: 6 }}>
            No orders yet
          </div>
          <div
            style={{
              fontSize: 14,
              color: "var(--plt-text-secondary)",
              marginBottom: 20,
            }}
          >
            You haven&apos;t placed any orders yet.
          </div>
          <Link href="/products" className="plt-btn-primary-lg">
            Browse products
          </Link>
        </div>
      ) : (
        <div className="plt-card-lg" style={{ padding: 0, overflow: "hidden" }}>
          {orders.map((o, i) => (
            <Link
              key={o.id}
              href={`/orders/${o.id}`}
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                padding: "18px 22px",
                borderBottom:
                  i < orders.length - 1
                    ? "1px solid var(--plt-border)"
                    : "none",
                textDecoration: "none",
                color: "var(--plt-text)",
                transition: "background 0.15s",
              }}
            >
              <div>
                <span
                  style={{
                    display: "inline-block",
                    borderRadius: 4,
                    padding: "3px 8px",
                    fontSize: 11,
                    fontWeight: 700,
                    background:
                      o.status === "CONFIRMED"
                        ? "var(--plt-green-bg-light)"
                        : o.status === "CANCELLED"
                          ? "var(--plt-error-bg)"
                          : "var(--plt-surface)",
                    color:
                      o.status === "CONFIRMED"
                        ? "var(--plt-green-text)"
                        : o.status === "CANCELLED"
                          ? "var(--plt-error)"
                          : "var(--plt-text-secondary)",
                  }}
                >
                  {o.status}
                </span>
                <div
                  style={{
                    fontSize: 13,
                    color: "var(--plt-text-secondary)",
                    marginTop: 4,
                  }}
                >
                  {formatDate(o.created_at)}
                </div>
              </div>
              <span style={{ fontWeight: 700, fontSize: 15 }}>
                {formatPrice(o.total_cents, o.currency)}
              </span>
            </Link>
          ))}
        </div>
      )}
    </main>
  );
}
