import { redirect } from "next/navigation";
import Link from "next/link";

import { GatewayError } from "@/lib/gateway";
import { getMe } from "@/lib/session";
import { logoutAction } from "@/app/(auth)/actions";
import { Container } from "@/components/ui/container";
import { Card } from "@/components/ui/card";
import { Button, buttonVariants } from "@/components/ui/button";
import { DeleteAccountButton } from "./delete-account";

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
    <Container as="main" size="md" className="pb-14 pt-10">
      <h1 className="mb-5 text-2xl font-extrabold">Your account</h1>

      <Card>
        <h2 className="mb-4 text-base font-extrabold">Account details</h2>
        <dl className="flex flex-col text-sm">
          <div className="flex justify-between border-b border-border py-2.5">
            <dt className="text-fg-muted">User ID</dt>
            <dd className="font-mono text-xs">{me.user_id}</dd>
          </div>
          <div className="flex justify-between border-b border-border py-2.5">
            <dt className="text-fg-muted">Role</dt>
            <dd className="font-semibold">{me.role}</dd>
          </div>
        </dl>

        <div className="mt-6 flex flex-col gap-3 sm:flex-row">
          <Link href="/orders" className={buttonVariants({ size: "lg" }) + " flex-1"}>
            View orders
          </Link>
          <Link href="/account/addresses" className={buttonVariants({ variant: "outline", size: "lg" }) + " flex-1"}>
            Manage addresses
          </Link>
          <form action={logoutAction} className="flex-1">
            <Button type="submit" variant="outline" size="lg" fullWidth>
              Log out
            </Button>
          </form>
        </div>
      </Card>

      <Card className="mt-5">
        <h2 className="mb-1.5 text-base font-extrabold">Data &amp; privacy</h2>
        <p className="mb-4 text-sm text-fg-muted">
          Download a machine-readable copy of your personal data — your profile, addresses, and
          orders. See our{" "}
          <Link href="/privacy" className="text-brand hover:underline">
            Privacy Policy
          </Link>{" "}
          for how we handle it.
        </p>
        {/* A plain <a> (not next/link) so the browser downloads the file. */}
        <a href="/account/export" className={buttonVariants({ variant: "outline" })}>
          Export my data (JSON)
        </a>

        <div className="mt-5 border-t border-border pt-4">
          <div className="mb-2 text-sm font-bold">Delete account</div>
          <DeleteAccountButton />
        </div>
      </Card>
    </Container>
  );
}
