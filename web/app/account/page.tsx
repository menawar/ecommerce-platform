import { redirect } from "next/navigation";
import Link from "next/link";

import { GatewayError } from "@/lib/gateway";
import { getMe } from "@/lib/session";
import { logoutAction } from "@/app/(auth)/actions";

// Protected page. The proxy already bounced anyone without a session cookie; here
// we do the REAL check by calling /me through the gateway (which validates the
// JWT). An expired/invalid token surfaces as a 401 -> back to login. This is the
// "edge gate is coarse, backend is authoritative" split in action.
export default async function AccountPage() {
  let me;
  try {
    me = await getMe();
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err;
  }

  return (
    <main style={{ maxWidth: 640, margin: "0 auto", padding: "40px 20px 60px" }}>
      <h1 style={{ fontSize: 26, fontWeight: 800, marginBottom: 18, marginTop: 0 }}>
        Your account
      </h1>

      <div className="plt-card-lg">
        <div style={{ fontSize: 16, fontWeight: 800, marginBottom: 16 }}>
          Account details
        </div>
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            gap: 12,
            fontSize: 14,
          }}
        >
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              padding: "10px 0",
              borderBottom: "1px solid var(--plt-border)",
            }}
          >
            <span style={{ color: "var(--plt-text-secondary)" }}>User ID</span>
            <span style={{ fontFamily: "monospace", fontSize: 12 }}>
              {me.user_id}
            </span>
          </div>
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              padding: "10px 0",
              borderBottom: "1px solid var(--plt-border)",
            }}
          >
            <span style={{ color: "var(--plt-text-secondary)" }}>Role</span>
            <span style={{ fontWeight: 600 }}>{me.role}</span>
          </div>
        </div>

        <div
          style={{
            display: "flex",
            gap: 12,
            marginTop: 24,
          }}
        >
          <Link
            href="/orders"
            className="plt-btn-primary-lg"
            style={{ textDecoration: "none", textAlign: "center", flex: 1 }}
          >
            View orders
          </Link>
          <form action={logoutAction} style={{ flex: 1 }}>
            <button className="plt-btn-outline" style={{ width: "100%" }}>
              Log out
            </button>
          </form>
        </div>
      </div>
    </main>
  );
}
