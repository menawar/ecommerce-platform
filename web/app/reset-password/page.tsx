import Link from "next/link";

import { ResetForm } from "./reset-form";

// Server Component. With a ?token= it shows the new-password form (which consumes
// the token only on POST). Without a token the link is malformed/old. searchParams
// is async in the App Router (Next 16).
export default async function ResetPasswordPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string }>;
}) {
  const { token } = await searchParams;

  return (
    <main style={{ maxWidth: 420, margin: "0 auto", padding: "60px 20px" }}>
      <div className="plt-card-lg" style={{ borderRadius: "var(--plt-radius-xl)", padding: "36px 32px" }}>
        <h1 style={{ fontSize: 22, fontWeight: 800, textAlign: "center", margin: 0 }}>Choose a new password</h1>
        {token ? (
          <>
            <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", textAlign: "center", margin: "6px 0 24px" }}>
              Enter a new password for your account.
            </p>
            <ResetForm token={token} />
          </>
        ) : (
          <>
            <p style={{ fontSize: 14, color: "var(--plt-text-secondary)", textAlign: "center", margin: "6px 0 24px" }}>
              This reset link is missing its token or is no longer valid.
            </p>
            <Link
              href="/forgot-password"
              className="plt-btn-primary-lg"
              style={{ display: "block", textDecoration: "none", textAlign: "center" }}
            >
              Request a new link
            </Link>
          </>
        )}
      </div>
    </main>
  );
}
