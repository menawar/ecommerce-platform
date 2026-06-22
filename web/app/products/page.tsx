import Link from "next/link";
import { listProducts } from "@/lib/gateway";
import { formatPrice } from "@/lib/format";

const PAGE_SIZE = 12;

// A Server Component: it runs on the server, awaits the gateway call directly, and
// streams HTML to the browser. searchParams is async in the App Router (Next 16).
export default async function ProductsPage({
  searchParams,
}: {
  searchParams: Promise<{ q?: string; page?: string }>;
}) {
  const sp = await searchParams;
  const q = sp.q ?? "";
  const page = Math.max(1, Number(sp.page) || 1);

  const { products, total } = await listProducts({ q, page, pageSize: PAGE_SIZE });
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <main className="mx-auto max-w-5xl px-6 py-10">
      <h1 className="text-2xl font-semibold">Products</h1>

      {/* A plain GET <form>: submitting navigates to /products?q=… — full-text
          search with ZERO client JavaScript. The server re-renders with the filter.
          This is the "you often don't need a Client Component" lesson. */}
      <form action="/products" className="mt-4 flex gap-2">
        <input
          type="search"
          name="q"
          defaultValue={q}
          placeholder="Search products…"
          className="w-full rounded-md border border-zinc-300 px-3 py-2"
        />
        <button className="rounded-md bg-foreground px-4 py-2 font-medium text-background">
          Search
        </button>
      </form>

      {products.length === 0 ? (
        <p className="mt-10 text-zinc-600">No products found.</p>
      ) : (
        <ul className="mt-6 grid grid-cols-2 gap-4 sm:grid-cols-3">
          {products.map((p) => (
            <li key={p.id} className="rounded-lg border border-zinc-200 p-4 hover:border-zinc-400">
              <Link href={`/products/${p.id}`} className="block">
                <h2 className="font-medium">{p.name}</h2>
                <p className="mt-1 text-sm text-zinc-500">{formatPrice(p.price_cents, p.currency)}</p>
                <p className="mt-2 text-xs text-zinc-400">
                  {p.available > 0 ? `${p.available} in stock` : "Out of stock"}
                </p>
              </Link>
            </li>
          ))}
        </ul>
      )}

      <nav className="mt-8 flex items-center justify-between text-sm">
        <PageLink q={q} page={page - 1} disabled={page <= 1}>
          ← Prev
        </PageLink>
        <span className="text-zinc-500">
          Page {page} of {totalPages} · {total} items
        </span>
        <PageLink q={q} page={page + 1} disabled={page >= totalPages}>
          Next →
        </PageLink>
      </nav>
    </main>
  );
}

function PageLink({
  q,
  page,
  disabled,
  children,
}: {
  q: string;
  page: number;
  disabled: boolean;
  children: React.ReactNode;
}) {
  if (disabled) return <span className="text-zinc-300">{children}</span>;
  const qs = new URLSearchParams();
  if (q) qs.set("q", q);
  qs.set("page", String(page));
  return (
    <Link href={`/products?${qs}`} className="font-medium underline">
      {children}
    </Link>
  );
}
