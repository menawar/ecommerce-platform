import Link from "next/link";

// not-found.tsx renders when notFound() is thrown in this segment (our detail page
// throws it on a gateway 404). Next also injects <meta robots="noindex"> so search
// engines skip it even though a streamed response carries a 200 status.
export default function NotFound() {
  return (
    <main className="mx-auto max-w-3xl px-6 py-10">
      <h1 className="text-xl font-semibold">Product not found</h1>
      <p className="mt-2 text-zinc-600">
        That product doesn&apos;t exist or is no longer available.
      </p>
      <Link href="/products" className="mt-4 inline-block font-medium underline">
        ← Back to products
      </Link>
    </main>
  );
}
