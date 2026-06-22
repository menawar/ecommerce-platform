import { redirect } from "next/navigation";

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
    <main className="mx-auto max-w-3xl px-6 py-10">
      <h1 className="text-2xl font-semibold">Your account</h1>
      <dl className="mt-6 space-y-2 text-sm">
        <div>
          <dt className="text-zinc-500">User ID</dt>
          <dd className="font-mono">{me.user_id}</dd>
        </div>
        <div>
          <dt className="text-zinc-500">Role</dt>
          <dd>{me.role}</dd>
        </div>
      </dl>

      <form action={logoutAction} className="mt-8">
        <button className="rounded-md border border-zinc-300 px-4 py-2 font-medium">
          Log out
        </button>
      </form>
    </main>
  );
}
