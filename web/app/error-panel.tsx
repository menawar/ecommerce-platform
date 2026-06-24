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
    <div className="rounded-lg border border-red-200 bg-red-50 p-6">
      <h2 className="text-lg font-semibold text-red-800">{title}</h2>
      <p className="mt-1 text-red-700">{message}</p>
      {requestId && (
        // Monospace + muted: it's a support/debugging handle, not primary content.
        // A user can quote it; we can grep the gateway logs/trace for the same id.
        <p className="mt-3 font-mono text-xs text-red-500">Reference: {requestId}</p>
      )}
    </div>
  );
}
