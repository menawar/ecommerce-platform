import type { ReactNode } from "react";

// LegalPage is the shared prose shell for the Terms and Privacy pages: a centered,
// readable column with a title and "last updated" line.
export function LegalPage({
  title,
  lastUpdated,
  children,
}: {
  title: string;
  lastUpdated: string;
  children: ReactNode;
}) {
  return (
    <main style={{ maxWidth: 760, margin: "0 auto", padding: "40px 20px 64px" }}>
      <h1 style={{ fontSize: 32, fontWeight: 800, marginBottom: 4 }}>{title}</h1>
      <p style={{ fontSize: 13, color: "var(--plt-text-muted)", marginBottom: 28 }}>
        Last updated {lastUpdated}
      </p>
      <div style={{ fontSize: 15, lineHeight: 1.75 }}>{children}</div>
    </main>
  );
}

// Section is a titled block used inside a LegalPage.
export function Section({ heading, children }: { heading: string; children: ReactNode }) {
  return (
    <section style={{ marginBottom: 24 }}>
      <h2 style={{ fontSize: 19, fontWeight: 700, margin: "0 0 8px" }}>{heading}</h2>
      {children}
    </section>
  );
}
