import { redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { getMe } from "@/lib/session";
import { listShippingMethods } from "@/lib/shipping";
import { ErrorPanel } from "../../error-panel";
import { ShippingManager } from "./shipping-manager";

// Admin shipping-method management. Role-gated like the products admin page (the
// gateway also enforces admin on the mutations; checking here avoids rendering a
// form a non-admin can't use). As admin, listShippingMethods returns ALL methods.
export default async function AdminShippingPage() {
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

  let methods;
  try {
    methods = await listShippingMethods(true); // admin view: include disabled methods
  } catch (err) {
    if (err instanceof GatewayError) {
      return (
        <main style={{ maxWidth: 720, margin: "0 auto", padding: "32px 20px" }}>
          <h1 style={{ fontSize: 22, fontWeight: 800 }}>Admin · Shipping</h1>
          <div style={{ marginTop: 20 }}>
            <ErrorPanel message={`Couldn't load shipping methods: ${err.message}`} requestId={err.requestId} />
          </div>
        </main>
      );
    }
    throw err;
  }

  return (
    <main style={{ maxWidth: 720, margin: "0 auto", padding: "32px 20px 60px" }}>
      <h1 style={{ fontSize: 22, fontWeight: 800, marginBottom: 24 }}>Admin · Shipping</h1>
      <ShippingManager methods={methods} />
    </main>
  );
}
