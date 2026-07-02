import { redirect } from "next/navigation";
import Link from "next/link";

import { GatewayError } from "@/lib/gateway";
import { listAddresses } from "@/lib/addresses";
import { Container } from "@/components/ui/container";
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
    <Container as="main" size="md" className="max-w-[720px] pb-14 pt-10">
      <Link href="/account" className="text-sm font-semibold text-accent hover:underline">
        ← Account
      </Link>
      <h1 className="mb-5 mt-3 text-2xl font-extrabold">Your addresses</h1>
      <AddressManager addresses={addresses} />
    </Container>
  );
}
