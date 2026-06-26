"use client";

// error.tsx MUST be a Client Component — error boundaries need React state and an
// onClick handler. It now catches only UNEXPECTED throws: the page handles
// GatewayError inline (and shows the gateway requestId there). Note `error.digest`
// — for errors crossing the server→client boundary, Next strips custom fields and,
// in production, replaces the message with a generic string, leaving only this
// server-generated digest as the correlation handle (it appears in the server
// logs). That's exactly why the gateway requestId is surfaced via inline
// catch-and-render, not here.
export default function Error({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <main style={{ maxWidth: 640, margin: "0 auto", padding: "60px 20px" }}>
      <div
        style={{
          background: "var(--plt-card)",
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
            background: "var(--plt-error-bg)",
            color: "var(--plt-error)",
            fontSize: 38,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            margin: "0 auto 20px",
          }}
        >
          !
        </div>
        <h1
          style={{
            fontSize: 22,
            fontWeight: 800,
            marginBottom: 8,
            marginTop: 0,
          }}
        >
          Something went wrong
        </h1>
        <p
          style={{
            fontSize: 14,
            color: "var(--plt-text-secondary)",
            marginBottom: 6,
          }}
        >
          {error.message}
        </p>
        {error.digest && (
          <p
            style={{
              fontFamily: "monospace",
              fontSize: 11,
              color: "var(--plt-text-muted)",
              marginBottom: 20,
            }}
          >
            Reference: {error.digest}
          </p>
        )}
        <button
          onClick={() => reset()}
          className="plt-btn-outline"
          style={{ width: "auto", padding: "10px 24px" }}
        >
          Try again
        </button>
      </div>
    </main>
  );
}
