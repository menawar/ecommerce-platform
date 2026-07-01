import Link from "next/link";
import { Container } from "@/components/ui/container";

export function Footer() {
  return (
    <footer className="bg-brand-deep pb-8 pt-10 text-[color:var(--plt-footer-text)]">
      <Container className="grid grid-cols-2 gap-7 md:grid-cols-[1.6fr_1fr_1fr_1fr]">
        {/* Brand column */}
        <div className="col-span-2 md:col-span-1">
          <div className="mb-3 flex items-center gap-2">
            <svg width="24" height="24" viewBox="0 0 32 32" fill="none" aria-hidden>
              <path d="M1 25 L11 10 L17 18.5 L22 11 L31 25 Z" fill="#7fb56a" />
              <circle cx="24.5" cy="8" r="3.2" fill="#f3b73f" />
            </svg>
            <span className="text-xl font-extrabold text-white">
              plateau<span className="text-[#e0894f]">.</span>
            </span>
          </div>
          <p className="max-w-[270px] text-[13px] leading-relaxed text-[color:var(--plt-footer-muted)]">
            Raw food materials, fresh from the Jos Plateau. Connecting Plateau farms and co-ops
            directly to your kitchen.
          </p>
          <p className="mt-3.5 text-xs text-[color:var(--plt-footer-dim)]">
            Farin Gada Market, Jos North, Plateau State
          </p>
        </div>

        {/* Shop column */}
        <nav className="text-[13px] leading-8" aria-label="Shop">
          <b className="text-white">Shop</b>
          <br />
          <Link href="/products" className="hover:underline">
            This week&apos;s harvest
          </Link>
          <br />
          <Link href="/products" className="hover:underline">
            Bulk orders
          </Link>
          <br />
          <Link href="/products" className="hover:underline">
            All produce
          </Link>
        </nav>

        {/* Farms column */}
        <div className="text-[13px] leading-8">
          <b className="text-white">Our farms</b>
          <br />
          Vom Farms
          <br />
          Riyom Growers
          <br />
          Barkin Ladi
        </div>

        {/* Help column */}
        <nav className="text-[13px] leading-8" aria-label="Help">
          <b className="text-white">Help</b>
          <br />
          <Link href="/orders" className="hover:underline">
            Track order
          </Link>
          <br />
          Delivery areas
          <br />
          Contact
        </nav>
      </Container>

      {/* Copyright bar */}
      <Container className="mt-4 flex flex-wrap justify-between gap-2 border-t border-[color:var(--plt-footer-rule)] pt-4 text-xs text-[color:var(--plt-footer-dim)]">
        <span>© 2026 Plateau · Home of Peace &amp; Tourism</span>
        <span className="flex flex-wrap gap-4">
          <Link href="/terms" className="underline">
            Terms
          </Link>
          <Link href="/privacy" className="underline">
            Privacy
          </Link>
          <span>Naira (₦) · Delivering across Jos &amp; Plateau</span>
        </span>
      </Container>
    </footer>
  );
}
