import { redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { getMe } from "@/lib/session";
import { listAllOrders } from "@/lib/orders";
import { formatPrice } from "@/lib/format";
import { ErrorPanel } from "../../error-panel";
import { shipOrderAction, deliverOrderAction, refundOrderAction } from "./actions";

// Admin fulfillment console: every order, with ship/deliver actions on the ones in
// the right state. Role-gated like the other admin pages (the gateway also enforces
// admin on the mutations).
export default async function AdminOrdersPage() {
  let role: string;
  try {
    role = (await getMe()).role;
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err;
  }
  if (role !== "admin") {
    return (
      <main style={{ maxWidth: 720, margin: "0 auto", padding: "32px 20px" }}>
        <ErrorPanel message="Admins only — you don't have access to this page." />
      </main>
    );
  }

  let orders;
  try {
    orders = await listAllOrders();
  } catch (err) {
    if (err instanceof GatewayError) {
      return (
        <main style={{ maxWidth: 900, margin: "0 auto", padding: "32px 20px" }}>
          <h1 style={{ fontSize: 22, fontWeight: 800 }}>Admin · Orders</h1>
          <div style={{ marginTop: 20 }}>
            <ErrorPanel message={`Couldn't load orders: ${err.message}`} requestId={err.requestId} />
          </div>
        </main>
      );
    }
    throw err;
  }

  return (
    <main style={{ maxWidth: 900, margin: "0 auto", padding: "32px 20px 60px" }}>
      <h1 style={{ fontSize: 22, fontWeight: 800, marginBottom: 24 }}>Admin · Orders</h1>

      {orders.length === 0 ? (
        <p style={{ fontSize: 14, color: "var(--plt-text-secondary)" }}>No orders yet.</p>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {orders.map((o) => (
            <div key={o.id} className="plt-card-lg" style={{ display: "flex", justifyContent: "space-between", gap: 16, alignItems: "center", flexWrap: "wrap" }}>
              <div style={{ fontSize: 13, lineHeight: 1.5 }}>
                <div style={{ fontFamily: "monospace", fontSize: 12 }}>{o.id}</div>
                <div style={{ color: "var(--plt-text-secondary)" }}>
                  {formatPrice(o.total_cents, o.currency)} · {o.shipping_method_name || "—"}
                  {o.tracking_number ? ` · ${o.tracking_number}` : ""}
                </div>
              </div>

              <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                <span
                  style={{
                    fontSize: 12,
                    fontWeight: 700,
                    borderRadius: 4,
                    padding: "2px 8px",
                    background: "var(--plt-surface-alt)",
                  }}
                >
                  {o.status}
                </span>

                {o.status === "CONFIRMED" && (
                  <form action={shipOrderAction} style={{ display: "flex", gap: 6 }}>
                    <input type="hidden" name="id" value={o.id} />
                    <input name="tracking_number" placeholder="Tracking (optional)" className="plt-input" style={{ fontSize: 13, padding: "4px 8px" }} />
                    <button className="plt-btn-outline">Mark shipped</button>
                  </form>
                )}
                {o.status === "SHIPPED" && (
                  <form action={deliverOrderAction}>
                    <input type="hidden" name="id" value={o.id} />
                    <button className="plt-btn-outline">Mark delivered</button>
                  </form>
                )}
                {(o.status === "PAID" || o.status === "CONFIRMED" || o.status === "SHIPPED" || o.status === "DELIVERED") && (
                  <form action={refundOrderAction}>
                    <input type="hidden" name="id" value={o.id} />
                    <button className="plt-btn-outline" style={{ color: "var(--plt-error)" }}>Refund</button>
                  </form>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </main>
  );
}
