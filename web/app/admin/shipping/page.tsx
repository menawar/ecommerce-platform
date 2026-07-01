import { GatewayError } from "@/lib/gateway";
import { listShippingMethods } from "@/lib/shipping";
import { ErrorPanel } from "../../error-panel";
import { adminGuard } from "../guard";
import { ShippingManager } from "./shipping-manager";

// Admin shipping-method management. adminGuard handles the role gate (the gateway
// also enforces admin on the mutations). As admin, listShippingMethods returns ALL.
export default async function AdminShippingPage() {
  const deny = await adminGuard();
  if (deny) return deny;

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
