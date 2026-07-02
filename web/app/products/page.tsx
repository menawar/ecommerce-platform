import type { Metadata } from "next";
import Link from "next/link";
import { listProducts, GatewayError } from "@/lib/gateway";
import { ErrorPanel } from "../error-panel";
import { SortSelect } from "./sort-select";
import { Container } from "@/components/ui/container";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Button, buttonVariants } from "@/components/ui/button";
import { ProductCard } from "@/components/product-card";

export const metadata: Metadata = {
  title: "Shop",
  description: "Browse this week's harvest — raw food materials fresh from the Jos Plateau.",
  alternates: { canonical: "/products" },
};

const PAGE_SIZE = 12;

// value = the backend sort key (server normalizes "featured"/unknown -> newest
// first). "Top rated" is intentionally absent: there's no ratings data yet, so it
// would be another dropdown that does nothing.
const SORT_OPTIONS = [
  { value: "featured", label: "Featured" },
  { value: "price_asc", label: "Price: Low to High" },
  { value: "price_desc", label: "Price: High to Low" },
];

// A Server Component: it runs on the server, awaits the gateway call directly, and
// streams HTML to the browser. searchParams is async in the App Router (Next 16).
export default async function ProductsPage({
  searchParams,
}: {
  searchParams: Promise<{ q?: string; page?: string; sort?: string }>;
}) {
  const sp = await searchParams;
  const q = sp.q ?? "";
  const page = Math.max(1, Number(sp.page) || 1);
  const sort = sp.sort ?? "featured";

  // Catch the gateway error HERE rather than letting it throw to error.tsx: a
  // Server Component can render the requestId into HTML directly, whereas the
  // error boundary would only receive a redacted error (no requestId, no message
  // in production). Anything that ISN'T a GatewayError is genuinely unexpected —
  // rethrow it and let the error boundary catch it.
  let products, total;
  try {
    ({ products, total } = await listProducts({ q, page, sort, pageSize: PAGE_SIZE }));
  } catch (err) {
    if (err instanceof GatewayError) {
      return (
        <Container as="main" className="pb-12 pt-4">
          <h1 className="text-xl font-extrabold">Products</h1>
          <div className="mt-6">
            <ErrorPanel message={`Couldn't load products: ${err.message}`} requestId={err.requestId} />
          </div>
        </Container>
      );
    }
    throw err;
  }
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <Container as="main" className="pb-12 pt-4">
      {/* Breadcrumb */}
      <nav aria-label="Breadcrumb" className="mb-4 text-sm text-fg-muted">
        <Link href="/" className="hover:underline">
          Home
        </Link>{" "}
        › <b className="text-fg">{q ? `Search: "${q}"` : "All produce"}</b> &nbsp;·&nbsp;{" "}
        <b className="text-fg">{total}</b> results
      </nav>

      <div className="flex flex-col gap-5 lg:flex-row lg:items-start">
        {/* Sidebar — desktop only; on mobile the header search + (Phase C) filter drawer cover this. */}
        <Card as="aside" className="hidden w-full lg:block lg:w-[230px] lg:flex-none">
          <div className="mb-2.5 text-sm font-extrabold">Search</div>
          <form action="/products" className="flex flex-col gap-2.5 border-b border-border-strong pb-4">
            {sort && sort !== "featured" && <input type="hidden" name="sort" value={sort} />}
            <Input type="search" name="q" defaultValue={q} placeholder="Search products…" aria-label="Search products" />
            <Button type="submit" fullWidth>
              Search
            </Button>
          </form>

          {/* Placeholder facets — become real in Phase C (rating + farm filters). */}
          <div className="pb-4 pt-4 text-sm font-extrabold">Customer rating</div>
          <div className="flex flex-col gap-2 border-b border-border-strong pb-4 text-sm text-fg-subtle">
            <span>★★★★☆ &amp; up</span>
            <span>★★★☆☆ &amp; up</span>
          </div>
          <div className="pb-2.5 pt-4 text-sm font-extrabold">Farm / Co-op</div>
          <div className="flex flex-col gap-2 text-sm text-fg-muted">
            {["Vom Farms", "Bukuru Co-op", "Riyom Growers", "Barkin Ladi Farms"].map((farm) => (
              <span key={farm}>{farm}</span>
            ))}
          </div>
        </Card>

        {/* Main column */}
        <div className="min-w-0 flex-1">
          {/* Toolbar */}
          <Card padded={false} className="mb-4 flex flex-wrap items-center justify-between gap-2.5 px-4 py-3">
            <div className="text-sm text-fg-muted">
              Showing <b className="text-fg">{products.length}</b> products
              {q && (
                <>
                  {" "}
                  for <b className="text-fg">&quot;{q}&quot;</b>
                </>
              )}
            </div>
            <div className="flex items-center gap-2.5 text-sm">
              <span>Sort by</span>
              <form action="/products" className="inline">
                {q && <input type="hidden" name="q" value={q} />}
                <SortSelect
                  options={SORT_OPTIONS}
                  value={sort}
                  className="h-9 rounded-md border border-border-strong bg-card px-2 text-sm font-semibold"
                />
              </form>
            </div>
          </Card>

          {/* Empty state */}
          {products.length === 0 ? (
            <Card padded={false}>
              <EmptyState
                icon="🔍"
                title="No produce found"
                description="Try another search or clear it to see everything."
                action={
                  <Link href="/products" className={buttonVariants({ variant: "outline" })}>
                    Clear search
                  </Link>
                }
              />
            </Card>
          ) : (
            <>
              {/* Product grid — 2 cols on phones up to 4 on wide screens */}
              <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 xl:grid-cols-4">
                {products.map((p) => (
                  <ProductCard key={p.id} product={p} />
                ))}
              </div>

              {/* Pagination */}
              <nav
                aria-label="Pagination"
                className="mt-6 flex items-center justify-between rounded-xl border border-border bg-card px-4 py-3 text-sm"
              >
                <PageLink q={q} sort={sort} page={page - 1} disabled={page <= 1}>
                  ← Prev
                </PageLink>
                <span className="text-fg-muted">
                  Page {page} of {totalPages} · {total} items
                </span>
                <PageLink q={q} sort={sort} page={page + 1} disabled={page >= totalPages}>
                  Next →
                </PageLink>
              </nav>
            </>
          )}
        </div>
      </div>
    </Container>
  );
}

function PageLink({
  q,
  sort,
  page,
  disabled,
  children,
}: {
  q: string;
  sort: string;
  page: number;
  disabled: boolean;
  children: React.ReactNode;
}) {
  if (disabled) return <span className="text-fg-subtle">{children}</span>;
  const qs = new URLSearchParams();
  if (q) qs.set("q", q);
  // Keep the active sort across pages; "featured" is the default, so omit it.
  if (sort && sort !== "featured") qs.set("sort", sort);
  qs.set("page", String(page));
  return (
    <Link href={`/products?${qs}`} className="font-semibold text-accent hover:underline">
      {children}
    </Link>
  );
}
