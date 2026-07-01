"use client";

import { useEffect } from "react";
import { reportError } from "@/lib/report-error";

// global-error replaces the root layout when an error escapes it, so it must render
// its own <html>/<body>. It reports the error to the seam and offers a retry.
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    reportError(error, { boundary: "global" });
  }, [error]);

  return (
    <html lang="en">
      <body style={{ fontFamily: "system-ui, sans-serif", margin: 0 }}>
        <main style={{ maxWidth: 480, margin: "0 auto", padding: "80px 20px", textAlign: "center" }}>
          <h1 style={{ fontSize: 24, fontWeight: 800, marginBottom: 8 }}>Something went wrong</h1>
          <p style={{ color: "#555", marginBottom: 24 }}>
            An unexpected error occurred. Please try again — if it keeps happening, contact support.
          </p>
          <button
            type="button"
            onClick={reset}
            style={{
              background: "#1f5c3d",
              color: "#fff",
              border: "none",
              borderRadius: 8,
              padding: "10px 20px",
              fontWeight: 700,
              cursor: "pointer",
            }}
          >
            Try again
          </button>
        </main>
      </body>
    </html>
  );
}
