import Link from "next/link";

// not-found.tsx renders when notFound() is thrown in this segment (our detail page
// throws it on a gateway 404). Next also injects <meta robots="noindex"> so search
// engines skip it even though a streamed response carries a 200 status.
export default function NotFound() {
  return (
    <main style={{ maxWidth: 640, margin: "0 auto", padding: "60px 20px" }}>
      <div
        className="plt-card-lg"
        style={{
          borderRadius: "var(--plt-radius-xl)",
          padding: "44px 36px",
          textAlign: "center",
        }}
      >
        <div
          style={{
            width: 72,
            height: 72,
            borderRadius: "50%",
            background: "var(--plt-surface)",
            color: "var(--plt-text-secondary)",
            fontSize: 38,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            margin: "0 auto 20px",
          }}
        >
          ?
        </div>
        <h1
          style={{
            fontSize: 22,
            fontWeight: 800,
            marginBottom: 8,
            marginTop: 0,
          }}
        >
          Product not found
        </h1>
        <p
          style={{
            fontSize: 14,
            color: "var(--plt-text-secondary)",
            marginBottom: 24,
          }}
        >
          That product doesn&apos;t exist or is no longer available.
        </p>
        <Link href="/products" className="plt-btn-primary-lg">
          ← Back to products
        </Link>
      </div>
    </main>
  );
}
