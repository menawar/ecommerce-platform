import "server-only";

import { createHmac } from "node:crypto";

// simulatePayment is a DEV-ONLY bridge that mimics what Paystack does after a
// customer authorizes payment: it POSTs a signed webhook to the payment service,
// which verifies the (mock) transaction and drives the saga to its outcome. The
// mock provider decides succeeded/failed from the amount encoded in the reference
// (total ending in .13 => declined), exactly like the rest of the mock path — the
// button just triggers the callback.
//
// In production this file is never reached: real PSP authorization_urls are
// absolute, so the BFF redirects to the PSP and the real webhook calls the backend.
const WEBHOOK_URL =
  process.env.PAYMENT_WEBHOOK_URL ?? "http://localhost:2115/webhooks/paystack";
const WEBHOOK_SECRET = process.env.PAYSTACK_WEBHOOK_SECRET ?? "dev-webhook-secret";

export async function simulatePayment(reference: string): Promise<void> {
  // Sign the EXACT bytes we send: the server recomputes HMAC-SHA512 over the raw
  // body, so any re-serialization would break verification.
  const body = JSON.stringify({ event: "charge.success", data: { reference } });
  const signature = createHmac("sha512", WEBHOOK_SECRET).update(body).digest("hex");

  const res = await fetch(WEBHOOK_URL, {
    method: "POST",
    headers: { "content-type": "application/json", "x-paystack-signature": signature },
    body,
    cache: "no-store",
  });
  if (!res.ok) {
    throw new Error(`mock webhook failed: ${res.status}`);
  }
}
