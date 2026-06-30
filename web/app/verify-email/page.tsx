import Link from "next/link";

import { isLoggedIn, getMe } from "@/lib/session";
import { VerifyConfirm } from "./verify-confirm";
import { ResendButton } from "./resend-button";

// Server Component. With a ?token= it renders a confirm step (VerifyConfirm), which
// consumes the token only on an explicit POST — never on this GET render, so email
// link-scanners can't burn it. Without a token it's the "check your inbox" landing
// page that registration / requireVerified send people to. searchParams is async
// in the App Router (Next 16).
export default async function VerifyEmailPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string }>;
}) {
  const { token } = await searchParams;
  const loggedIn = await isLoggedIn();

  // On the no-token landing, skip the resend offer for a user who's already
  // verified — otherwise the server no-ops and the UI would falsely say "Sent".
  let alreadyVerified = false;
  if (!token && loggedIn) {
    try {
      alreadyVerified = (await getMe()).email_verified;
    } catch {
      alreadyVerified = false;
    }
  }

  return (
    <main style={{ maxWidth: 480, margin: "0 auto", padding: "60px 20px" }}>
      <div className="plt-card-lg" style={{ borderRadius: "var(--plt-radius-xl)", padding: "36px 32px", textAlign: "center" }}>
        {token ? (
          <VerifyConfirm token={token} loggedIn={loggedIn} />
        ) : alreadyVerified ? (
          <>
            <h1 style={{ fontSize: 22, fontWeight: 800, margin: 0 }}>Email already verified</h1>
            <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", margin: "10px 0 0" }}>
              Your email is confirmed — you’re all set.
            </p>
            <Link
              href="/products"
              className="plt-btn-primary-lg"
              style={{ display: "block", textDecoration: "none", marginTop: 24 }}
            >
              Continue shopping
            </Link>
          </>
        ) : (
          <>
            <h1 style={{ fontSize: 22, fontWeight: 800, margin: 0 }}>Verify your email</h1>
            <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", margin: "10px 0 0" }}>
              We sent a verification link to your email. Click it to confirm your
              address and unlock checkout.
            </p>
            {loggedIn ? (
              <ResendButton />
            ) : (
              <Link
                href="/login"
                className="plt-btn-primary-lg"
                style={{ display: "block", textDecoration: "none", marginTop: 24 }}
              >
                Sign in
              </Link>
            )}
          </>
        )}
      </div>
    </main>
  );
}
