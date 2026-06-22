// loading.tsx is rendered instantly (via React Suspense) while the Server
// Component above awaits its data — so navigation feels immediate.
export default function Loading() {
  return <main className="mx-auto max-w-5xl px-6 py-10 text-zinc-500">Loading…</main>;
}
