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
    <main className="mx-auto max-w-5xl px-6 py-10">
      <h1 className="text-xl font-semibold">Something went wrong</h1>
      <p className="mt-2 text-zinc-600">{error.message}</p>
      {error.digest && (
        <p className="mt-2 font-mono text-xs text-zinc-400">Reference: {error.digest}</p>
      )}
      <button
        onClick={() => reset()}
        className="mt-4 rounded-md border border-zinc-300 px-4 py-2 font-medium"
      >
        Try again
      </button>
    </main>
  );
}
