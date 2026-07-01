# UI/UX Upgrade Plan — world-class storefront

Goal: raise `web/` from a functional storefront to a **world-standard** shopping
experience with the depth of Amazon / eBay / Alibaba — while keeping the Plateau
brand (Jos-Plateau farm-to-kitchen, NGN) and the BFF architecture.

Built phase-by-phase in the repo's rhythm: **one increment per PR**, tests ship
with each, `/code-review` before merge, feature branch first.

---

## 1. Current-state assessment (grounded)

| Area | Today | Gap to world-class |
|---|---|---|
| Styling | Tailwind v4 imported but unused; `--plt-*` tokens + ~20 utility classes + **518 inline `style={{}}`** | No component system; unmaintainable; inconsistent spacing/typography |
| Responsive | **1 media query**; fixed widths (`flex: 0 0 380px`) | Not mobile-first; likely broken on phones (mobile = majority of commerce traffic) |
| Product media | Plain `<img>` placeholder; fake thumbnails | Gallery, zoom, multiple images, video, `next/image` optimization |
| Reviews | Hard-coded `★★★★★` | Real ratings + reviews + photos + Q&A (needs backend) |
| Search/discovery | `?q=` + sort dropdown | Autocomplete, faceted filters, breadcrumbs, recently-viewed |
| Cart/checkout | Full-page, functional | Mini-cart drawer, save-for-later, coupons, trust signals, progress steps |
| Merchandising | Basic "related products" | Recommendations, personalized home, promo carousels, badges |
| Wishlist | None | Save/wishlist, share (needs backend) |
| Performance/a11y | Skeleton class exists; skip link added (15.3) | CWV budget, image optimization, WCAG AA, motion, empty/error states |

**Foundational truth:** almost everything below depends on a real **design
system + component library** replacing the 518 inline styles. That is Phase A and
must come first — it makes every later phase faster and consistent.

---

## 2. Guiding principles

1. **Mobile-first, responsive by default** — every component works 360px→wide.
2. **Design system over one-offs** — tokens → Tailwind theme → primitives → patterns; no new inline styles.
3. **Accessible (WCAG 2.2 AA)** — semantic, keyboard, focus, contrast, reduced-motion.
4. **Fast (Core Web Vitals green)** — `next/image`, streaming/Suspense, skeletons, minimal client JS (keep the Server-Component-first BFF).
5. **Incremental & shippable** — migrate page-by-page; never a big-bang rewrite; keep every PR green.
6. **Respect the architecture** — Server Components + server actions + httpOnly cookie; new data needs gateway routes → gRPC → services (db-per-service).
7. **Keep the brand** — evolve the Plateau look (greens/terracotta/gold), don't clone Amazon's chrome.

---

## 3. Benchmark features → our roadmap

| Benchmark (Amazon/eBay/Alibaba) | Where it lands |
|---|---|
| Fast faceted search + autocomplete | Phase C |
| Rich PDP: gallery, zoom, variants, A+ content | Phase D |
| Ratings, reviews, photos, Q&A | Phase E (+ backend) |
| Wishlist / save-for-later | Phase E/F (+ backend) |
| Mini-cart drawer, coupons, 1-page checkout | Phase F |
| "Customers also bought", recently viewed, personalized home | Phase G (+ backend) |
| Order tracking timeline, reorder, invoices | Phase H |
| Trust signals, urgency, social proof, PWA | Phase J |

---

## 4. Phased roadmap

### Phase A — Design-system foundation ★ (do first)
**Goal:** one source of truth for look & feel; kill inline styles.
- Wire the `--plt-*` tokens into the **Tailwind v4 theme** (`@theme`), add a type
  scale, spacing scale, radii, shadows, semantic color roles (bg/surface/fg/
  border/brand/danger/success), and dark-mode-ready variables.
- Build **primitives** (`web/components/ui/`): `Button`, `IconButton`, `Input`,
  `Select`, `Textarea`, `Checkbox`, `Radio`, `Card`, `Badge`, `Tag`, `Modal`/
  `Dialog`, `Drawer`/`Sheet`, `Toast`, `Tooltip`, `Skeleton`, `Spinner`,
  `Breadcrumbs`, `Pagination`, `Rating`, `Tabs`, `Accordion`, `EmptyState`.
- Adopt a **`cn()`/class-variance** pattern for variants; a `<Container>` + grid.
- **Migrate incrementally**: convert existing pages to primitives, deleting inline
  styles; fold in prior deferred cleanups (`InlineFormError`, admin guard).
- Storybook-lite: a `/style-guide` route (dev-only) rendering every primitive.
- **Backend:** none. **Acceptance:** primitives + tokens shipped; ≥1 flow migrated; visual regression stable.
- **PRs:** A1 tokens+Tailwind theme; A2 core primitives; A3 form primitives + migrate auth/checkout forms; A4 layout/nav primitives.

### Phase B — Responsive & mobile-first shell
**Goal:** flawless on phones.
- Responsive **header**: sticky, condensed on scroll; **category mega-menu**
  (desktop) + **slide-in drawer** (mobile); prominent search; cart badge.
- Mobile **bottom tab bar** (Home/Search/Cart/Account) optional.
- Responsive grids for catalog/PDP/cart; fluid typography; safe-area insets.
- Audit + fix every fixed-width layout.
- **Backend:** none. **Acceptance:** 360/768/1280 all clean; Lighthouse mobile ≥90 layout.
- **PRs:** B1 header/nav (mega-menu + drawer); B2 responsive catalog/PDP/cart grids.

### Phase C — Discovery & search
**Goal:** find products fast.
- **Search autocomplete** (typeahead: product/category suggestions) via a debounced
  gateway search route.
- **Faceted filters**: category, price range, availability, (later) rating; URL-
  synced; mobile filter drawer; result count; active-filter chips.
- **Breadcrumbs**, richer sort, **recently viewed** (localStorage), no-results state.
- **Backend:** Product service search/facets RPC (name/category/price filters +
  suggestions); gateway `/search`, `/search/suggest`. (Later: OpenSearch/Typesense if scale needs.)
- **Acceptance:** faceted search with autocomplete, URL-shareable. **PRs:** C1 backend search+facets; C2 filters UI; C3 autocomplete; C4 breadcrumbs + recently-viewed.

### Phase D — Product experience (PDP)
**Goal:** a PDP that sells.
- **Image gallery**: multiple images, thumbnails, hover/zoom, lightbox, `next/image`.
- **Variants/options** (size/weight/grade) with per-variant price/stock (if catalog supports).
- Sticky buy-box, stock/urgency ("only N left"), delivery estimate, quantity, share,
  add-to-cart → **toast + mini-cart**; specs/description tabs; trust row.
- **Backend:** Product multi-image + (optional) variants (schema + RPC + admin upload to MinIO/S3, which already exists).
- **Acceptance:** gallery + zoom + variants + rich buy-box. **PRs:** D1 product images (backend+admin+gallery); D2 buy-box/urgency/delivery; D3 variants (if in scope); D4 tabs/specs.

### Phase E — Ratings, reviews & wishlist
**Goal:** social proof + save-for-later.
- **Reviews**: star rating, title/body, verified-purchase badge, photos, helpful
  votes, sort/filter, rating histogram, aggregate on PDP + cards.
- **Wishlist / save-for-later**: heart on cards/PDP, wishlist page, move to cart.
- **Q&A** (optional).
- **Backend (significant):** a new **Review service** (db-per-service: reviews,
  votes; verified-purchase check via Order) + **Wishlist** (User service or its own);
  gateway routes; events. Enforce verified-purchase + moderation.
- **Acceptance:** real reviews drive PDP + search rating facet; wishlist persists.
- **PRs:** E1 review service+proto+gateway; E2 review UI (submit/list/aggregate); E3 wishlist backend+UI; E4 helpful-votes/photos/Q&A.

### Phase F — Cart & checkout UX
**Goal:** reduce friction, lift conversion.
- **Mini-cart drawer** (add-to-cart opens it); quantity/remove inline; **save-for-later**.
- **Coupons/promotions** (needs backend pricing rules); order-summary with savings.
- Checkout **progress steps**, address autocomplete, saved-address quick-select,
  delivery-date estimate, trust/secure-payment badges, clearer errors, sticky summary.
- **Backend:** coupon/promotion rules (Order/Product pricing); the rest is web + existing routes.
- **Acceptance:** mini-cart + coupon + polished checkout. **PRs:** F1 mini-cart drawer; F2 save-for-later; F3 coupons (backend+UI); F4 checkout polish.

### Phase G — Recommendations & merchandising
**Goal:** discovery + AOV.
- **Home**: hero, category tiles, "This week's harvest", deals, **personalized rows**.
- **PDP/cart**: "Customers also bought", "Frequently bought together", recently viewed.
- Promo **banners/carousels**, badges (New/Bestseller/Low-stock), collections/landing pages.
- **Backend:** a recommendations endpoint (start simple: co-purchase from Order data / popularity; later ML). Merchandising config (admin-curated collections).
- **Acceptance:** recs on home/PDP/cart; curated collections. **PRs:** G1 home revamp; G2 recs backend (popularity/co-purchase); G3 recs UI rows; G4 collections/promos.

### Phase H — Account & order experience
**Goal:** post-purchase delight + retention.
- **Order tracking timeline** (Placed→Confirmed→Shipped→Delivered) with tracking #;
  **reorder**, downloadable invoice/receipt, returns/refund request UI.
- Account dashboard (recent orders, addresses, wishlist, saved payment note).
- **Backend:** invoice generation; returns flow (extends refund).
- **Acceptance:** timeline + reorder + invoice. **PRs:** H1 order timeline; H2 reorder + invoice; H3 account dashboard.

### Phase I — Performance, accessibility & polish
**Goal:** fast, accessible, delightful (cross-cutting; run alongside).
- `next/image` everywhere (+ `remotePatterns` for MinIO/S3/R2), responsive sizes,
  blur placeholders; route-level **Suspense + skeletons**; bundle/CWV budget.
- **WCAG 2.2 AA** audit (axe): focus management, labels, contrast, keyboard, ARIA,
  `prefers-reduced-motion`; consistent **empty/error/loading** states.
- Micro-interactions (subtle transitions), toast system, optimistic UI where safe.
- **Backend:** none. **Acceptance:** Lighthouse ≥90 across PWA/Perf/A11y/SEO; axe clean.
- **PRs:** I1 image optimization; I2 Suspense/skeletons; I3 a11y audit fixes; I4 motion/empty-states.

### Phase J — Trust, conversion & PWA
**Goal:** credibility + re-engagement.
- Trust signals (secure-checkout, returns policy, ratings summary, "X sold"),
  urgency/scarcity done honestly, social proof.
- **PWA**: manifest, installable, offline shell, add-to-home; web-push for order
  updates (ties into Notification service).
- Newsletter/marketing opt-in (consent-aware), share/referral.
- **Backend:** web-push subscriptions (Notification service). **Acceptance:** installable PWA + trust surfaces. **PRs:** J1 PWA/manifest; J2 trust/social-proof; J3 web-push.

---

## 5. Backend work called out (not "just frontend")
World-class commerce needs data the current services don't have yet:
- **Product**: multiple images, variants/options, richer attributes, search/facets/suggest.
- **Review service** (new): ratings/reviews/votes/photos + verified-purchase.
- **Wishlist** (User or new svc).
- **Promotions/coupons** (pricing rules; Order/Product).
- **Recommendations** (co-purchase/popularity from Order; endpoint).
- **Invoices / returns** (Order).
- **Web-push** (Notification).
Each follows the platform rules: db-per-service, gRPC + gateway route, events, tests, `/code-review`.

## 6. Sequencing & effort (recommended)
**A → B → C → D** first (foundation, mobile, discovery, PDP — the highest-visibility
uplift with mostly-known backend). Then **E, F, G** (the deeper commerce features
needing new services). **H, J** for retention/trust. **I runs continuously**.
Rough size: A/B ~foundational (several PRs each), C/D/E/G each a mini-phase with
backend, F/H/I/J medium. Expect ~30–40 small PRs total.

## 7. Success metrics
- **CWV**: LCP <2.5s, INP <200ms, CLS <0.1 (mobile).
- **Lighthouse** ≥90 Perf/A11y/Best-Practices/SEO; **axe** zero criticals.
- **UX**: mobile-usable at 360px; keyboard-complete; consistent design tokens (0 new inline styles).
- **Commerce**: reviews on PDP, faceted search, wishlist, mini-cart, recs — feature-parity checklist vs. benchmarks.

## 8. Risks & mitigations
- **Big-bang temptation** → migrate page-by-page behind the primitives; keep green.
- **Scope creep on backend** → land the web/UX shell first with graceful fallbacks; add each service (reviews/recs/coupons) as its own gated mini-phase.
- **Perf regressions from client JS** → stay Server-Component-first; measure CWV per PR.
- **Design drift** → the `/style-guide` route + tokens are the guardrail.
