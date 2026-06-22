import Link from "next/link";

// The home page is a Server Component (the App Router default). No "use client"
// here — it renders to HTML on the server and ships zero JavaScript for this view.
export default function Home() {
  return (
    <main className="mx-auto max-w-3xl px-6 py-16">
      <h1 className="text-3xl font-semibold tracking-tight">E-Commerce</h1>
      <p className="mt-3 text-zinc-600">
        A storefront for the Go microservices platform. Browse the catalog, sign in,
        build a cart, and check out.
      </p>
      <Link
        href="/products"
        className="mt-8 inline-block rounded-md bg-foreground px-4 py-2 font-medium text-background"
      >
        Browse products
      </Link>
    </main>
  );
}
