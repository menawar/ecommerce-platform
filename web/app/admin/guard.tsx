import type { ReactNode } from "react";
import { redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { getMe } from "@/lib/session";
import { ErrorPanel } from "../error-panel";

// adminGuard is the shared gate for admin pages. It validates the session via /me:
// a lapsed session (401) redirects to login; a non-admin gets an "Admins only"
// panel to render; an admin gets null (proceed). Usage in a Server Component:
//
//	const deny = await adminGuard();
//	if (deny) return deny;
//
// The gateway independently enforces admin on the mutations — this just avoids
// rendering a management UI a non-admin can't use.
export async function adminGuard(): Promise<ReactNode | null> {
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
  return null;
}
