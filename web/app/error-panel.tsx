// A reusable, presentational error panel — a plain Server Component (no "use
// client", no hooks). It's rendered INLINE by a Server Component that caught a
// GatewayError, which is the whole trick: because the Server Component reads the
// error's fields in-process and bakes them straight into HTML, the requestId
// never has to survive React's server→client error serialization. An error.tsx
// boundary can't do this — Next strips custom fields (and, in production, the
// message) before the error reaches the client boundary, leaving only `digest`.
export function ErrorPanel({
  title = "Something went wrong",
  message,
  requestId,
}: {
  title?: string;
  message: string;
  requestId?: string;
}) {
  return (
    <div
      style={{
        background: "var(--plt-error-bg)",
        border: "1px solid var(--plt-error-border)",
        borderRadius: "var(--plt-radius-lg)",
        padding: "20px 24px",
      }}
    >
      <div
        style={{
          fontSize: 16,
          fontWeight: 700,
          color: "var(--plt-error)",
          marginBottom: 6,
        }}
      >
        {title}
      </div>
      <div style={{ fontSize: 14, color: "#b42318" }}>{message}</div>
      {requestId && (
        // Monospace + muted: it's a support/debugging handle, not primary content.
        // A user can quote it; we can grep the gateway logs/trace for the same id.
        <div
          style={{
            marginTop: 12,
            fontFamily: "monospace",
            fontSize: 11,
            color: "#d97560",
          }}
        >
          Reference: {requestId}
        </div>
      )}
    </div>
  );
}
