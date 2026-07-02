import Link from "next/link";
import { listProducts } from "@/lib/gateway";
import { Container } from "@/components/ui/container";
import { Card } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { buttonVariants } from "@/components/ui/button";
import { ProductCard } from "@/components/product-card";

// The home page is a Server Component (the App Router default). No "use client"
// here — it renders to HTML on the server and ships zero JavaScript for this view.
export default async function Home() {
  let products: Awaited<ReturnType<typeof listProducts>>["products"] = [];
  try {
    const result = await listProducts({ page: 1, pageSize: 12 });
    products = result.products;
  } catch {
    // Silently fall back to empty grid if the gateway is unavailable.
  }

  const deals = products.slice(0, 6);
  const fresh = products;

  return (
    <Container as="main" className="pb-12 pt-4">
      {/* Hero */}
      <div className="relative flex min-h-[280px] items-center overflow-hidden rounded-xl bg-[linear-gradient(180deg,var(--plt-hero-start)_0%,var(--plt-hero-end)_100%)] px-6 sm:min-h-[330px] sm:px-10 lg:px-14">
        <svg
          viewBox="0 0 640 330"
          preserveAspectRatio="xMidYMax slice"
          className="absolute inset-0 h-full w-full"
          aria-hidden
        >
          <circle cx="505" cy="88" r="46" fill="#f3b73f" opacity="0.9" />
          <path d="M0 248 Q140 178 300 216 T640 198 V330 H0 Z" fill="#d6e4c6" />
          <path d="M0 270 Q170 208 360 246 T640 234 V330 H0 Z" fill="#a7c98c" />
          <path d="M0 296 Q200 248 410 278 T640 272 V330 H0 Z" fill="#5f9a4d" />
          <path d="M0 318 Q220 290 460 308 T640 302 V330 H0 Z" fill="#3c6b34" />
        </svg>

        <div className="relative z-[1] max-w-[470px]">
          <div className="text-xs font-extrabold uppercase tracking-[.08em] text-accent">
            Jos Plateau · Home of Peace &amp; Tourism
          </div>
          <h1 className="my-3 text-3xl font-extrabold leading-[1.05] tracking-tight text-brand-deep sm:text-4xl lg:text-[44px]">
            Raw food, straight from the Plateau.
          </h1>
          <p className="mb-6 max-w-md text-base text-fg-muted">
            Irish potatoes, acha, sweet tomatoes — grown in the cool Jos highlands, harvested to order.
          </p>
          <Link href="/products" className={buttonVariants({ variant: "hero", size: "lg" })}>
            Shop the harvest
          </Link>
        </div>
      </div>

      {/* Top harvest deals */}
      {deals.length > 0 && (
        <Card className="mt-5">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-xl font-extrabold">Top harvest deals</h2>
            <Link href="/products" className="text-sm font-bold text-accent hover:underline">
              See all →
            </Link>
          </div>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6">
            {deals.map((p) => (
              <ProductCard key={p.id} product={p} />
            ))}
          </div>
        </Card>
      )}

      {/* Fresh for you */}
      {fresh.length > 0 && (
        <Card className="mt-5">
          <h2 className="mb-4 text-xl font-extrabold">Fresh for you</h2>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6">
            {fresh.map((p) => (
              <ProductCard key={p.id} product={p} />
            ))}
          </div>
        </Card>
      )}

      {/* Empty state */}
      {products.length === 0 && (
        <Card padded={false} className="mt-5">
          <EmptyState
            icon="🌿"
            title="Welcome to Plateau"
            description="Browse this week's harvest from the Jos Plateau."
            action={
              <Link href="/products" className={buttonVariants({ size: "lg" })}>
                Browse produce
              </Link>
            }
          />
        </Card>
      )}
    </Container>
  );
}
