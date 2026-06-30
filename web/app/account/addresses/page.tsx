import { redirect } from "next/navigation";
import Link from "next/link";

import { GatewayError } from "@/lib/gateway";
import { listAddresses } from "@/lib/addresses";
import { AddressManager } from "./address-manager";

// Protected: listAddresses goes through the gateway, which validates the JWT. A 401
// means the session lapsed → back to login (same pattern as the account page).
export default async function AddressesPage() {
  let addresses;
  try {
    addresses = await listAddresses();
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err;
  }

  return (
    <main style={{ maxWidth: 720, margin: "0 auto", padding: "40px 20px 60px" }}>
      <Link href="/account" style={{ fontSize: 13, color: "var(--plt-terracotta)", fontWeight: 600 }}>
        ← Account
      </Link>
      <h1 style={{ fontSize: 26, fontWeight: 800, margin: "12px 0 18px" }}>Your addresses</h1>
      <AddressManager addresses={addresses} />
    </main>
  );
}
