import { completeMockPaymentAction } from "./actions";

// Dev-only simulated PSP checkout page. The mock payment provider returns a
// relative authorization_url that lands here; "Complete payment" fires the
// simulated webhook (see lib/mock-psp). Whether the order then CONFIRMS or
// CANCELS is decided by the mock rule — a cart total ending in .13 (e.g. ₦13.13)
// declines — not by this button.
export default async function MockPayPage({
  params,
  searchParams,
}: {
  params: Promise<{ ref: string }>;
  searchParams: Promise<{ order?: string }>;
}) {
  const { ref } = await params;
  const { order } = await searchParams;

  return (
    <main style={{ maxWidth: 480, margin: "0 auto", padding: "60px 20px" }}>
      <div
        className="plt-card-lg"
        style={{ borderRadius: "var(--plt-radius-xl)", padding: "40px 32px", textAlign: "center" }}
      >
        <div style={{ fontSize: 13, fontWeight: 700, color: "var(--plt-text-secondary)", letterSpacing: 1 }}>
          MOCK PAYMENT PROVIDER
        </div>
        <h1 style={{ fontSize: 22, fontWeight: 800, margin: "10px 0 6px" }}>Authorize your payment</h1>
        <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", marginBottom: 24 }}>
          This stands in for the Paystack checkout page in local development.
          Clicking below sends the payment confirmation webhook.
        </p>

        <form action={completeMockPaymentAction}>
          <input type="hidden" name="reference" value={ref} />
          <input type="hidden" name="order_id" value={order ?? ""} />
          <button type="submit" className="plt-btn-primary-lg" style={{ width: "100%" }}>
            Complete payment
          </button>
        </form>

        <p style={{ fontSize: 12, color: "var(--plt-text-secondary)", marginTop: 18 }}>
          Tip: a cart total ending in <b>.13</b> simulates a declined card.
        </p>
      </div>
    </main>
  );
}
