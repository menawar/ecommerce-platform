"use client";

// error.tsx MUST be a Client Component — error boundaries need React state and an
// onClick handler. It catches anything thrown while rendering the route (e.g. the
// gateway being down) and offers a retry that re-runs the Server Component.
export default function Error({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <main className="mx-auto max-w-5xl px-6 py-10">
      <h1 className="text-xl font-semibold">Something went wrong</h1>
      <p className="mt-2 text-zinc-600">{error.message}</p>
      <button
        onClick={() => reset()}
        className="mt-4 rounded-md border border-zinc-300 px-4 py-2 font-medium"
      >
        Try again
      </button>
    </main>
  );
}
