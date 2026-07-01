import type { Metadata } from "next";
import { cn } from "@/lib/cn";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Rating } from "@/components/ui/rating";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { Input, Select } from "@/components/ui/input";
import { Field } from "@/components/ui/field";
import { DrawerDemo } from "./drawer-demo";

export const metadata: Metadata = {
  title: "Style guide",
  description: "Design-system reference: color roles, radii, elevation, type scale.",
  robots: { index: false, follow: false },
};

// A living reference for the Phase-A design system. It renders the semantic tokens
// via Tailwind utilities (bg-brand, text-fg-muted, rounded-xl, shadow-card…), so if
// this page looks right, the theme is wired right. Grows as primitives land (A2+).

const COLOR_ROLES = [
  ["bg-brand", "brand"],
  ["bg-brand-deep", "brand-deep"],
  ["bg-brand-subtle", "brand-subtle"],
  ["bg-accent", "accent"],
  ["bg-gold", "gold"],
  ["bg-surface", "surface"],
  ["bg-card", "card"],
  ["bg-danger", "danger"],
  ["bg-success", "success"],
  ["bg-star", "star"],
] as const;

const RADII = ["rounded-sm", "rounded-md", "rounded-lg", "rounded-xl", "rounded-pill"] as const;
const SHADOWS = ["shadow-card", "shadow-md", "shadow-lg"] as const;
const TEXT = [
  ["text-3xl font-extrabold", "Display / 3xl"],
  ["text-2xl font-bold", "Heading / 2xl"],
  ["text-xl font-bold", "Heading / xl"],
  ["text-base font-medium", "Body / base"],
  ["text-sm text-fg-muted", "Small / muted"],
  ["text-xs text-fg-subtle", "Caption / subtle"],
] as const;

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-10">
      <h2 className="mb-4 text-lg font-bold text-fg">{title}</h2>
      {children}
    </section>
  );
}

export default function StyleGuidePage() {
  return (
    <main className="mx-auto max-w-4xl px-5 py-10">
      <h1 className="mb-1 text-3xl font-extrabold text-fg">Style guide</h1>
      <p className="mb-10 text-sm text-fg-muted">
        Phase A design tokens. Use these semantic utilities in new components — no inline styles.
      </p>

      <Section title="Color roles">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-5">
          {COLOR_ROLES.map(([bg, name]) => (
            <div key={name} className="overflow-hidden rounded-lg border border-border">
              <div className={cn("h-16 w-full", bg)} />
              <div className="bg-card p-2 text-xs text-fg-muted">{name}</div>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Radii">
        <div className="flex flex-wrap gap-4">
          {RADII.map((r) => (
            <div key={r} className="text-center">
              <div className={cn("h-16 w-16 border border-border bg-brand-subtle", r)} />
              <div className="mt-1 text-xs text-fg-subtle">{r}</div>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Elevation">
        <div className="flex flex-wrap gap-6">
          {SHADOWS.map((s) => (
            <div key={s} className="text-center">
              <div className={cn("h-16 w-24 rounded-lg bg-card", s)} />
              <div className="mt-2 text-xs text-fg-subtle">{s}</div>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Typography">
        <div className="space-y-2">
          {TEXT.map(([cls, label]) => (
            <p key={label} className={cn(cls)}>
              {label} — Fresh from the Jos Plateau
            </p>
          ))}
        </div>
      </Section>

      <Section title="Buttons">
        <div className="flex flex-wrap items-center gap-3">
          <Button variant="primary">Primary</Button>
          <Button variant="hero">Hero</Button>
          <Button variant="gold">Gold</Button>
          <Button variant="outline">Outline</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="danger">Danger</Button>
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-3">
          <Button size="sm">Small</Button>
          <Button size="md">Medium</Button>
          <Button size="lg">Large</Button>
          <Button loading>Loading</Button>
          <Button disabled>Disabled</Button>
        </div>
      </Section>

      <Section title="Badges">
        <div className="flex flex-wrap gap-2">
          <Badge variant="neutral">Neutral</Badge>
          <Badge variant="brand">In stock</Badge>
          <Badge variant="gold">Bestseller</Badge>
          <Badge variant="danger">Low stock</Badge>
          <Badge variant="success">Delivered</Badge>
        </div>
      </Section>

      <Section title="Card + Rating">
        <Card className="max-w-sm">
          <div className="font-bold text-fg">Jos Plateau Potatoes</div>
          <div className="mt-1">
            <Rating value={4.5} count={128} />
          </div>
          <p className="mt-2 text-sm text-fg-muted">Freshly harvested, delivered this week.</p>
          <Button className="mt-4" fullWidth>
            Add to cart
          </Button>
        </Card>
      </Section>

      <Section title="Skeleton">
        <div className="max-w-sm space-y-2">
          <Skeleton className="h-32 w-full" />
          <Skeleton className="h-4 w-2/3" />
          <Skeleton className="h-4 w-1/3" />
        </div>
      </Section>

      <Section title="Form fields">
        <div className="grid max-w-md gap-4">
          <Field label="Email" htmlFor="sg-email" hint="We'll never share it.">
            <Input id="sg-email" type="email" placeholder="you@example.com" />
          </Field>
          <Field label="Quantity" htmlFor="sg-qty">
            <Select id="sg-qty" defaultValue="1">
              <option>1</option>
              <option>2</option>
              <option>3</option>
            </Select>
          </Field>
          <Field label="Coupon" htmlFor="sg-coupon" error="That code has expired.">
            <Input id="sg-coupon" invalid defaultValue="OLD2020" />
          </Field>
        </div>
      </Section>

      <Section title="Drawer">
        <DrawerDemo />
      </Section>

      <Section title="Empty state">
        <Card padded={false} className="max-w-md">
          <EmptyState
            icon="🧺"
            title="Your cart is empty"
            description="Browse this week's harvest and add something fresh."
            action={<Button variant="outline">Shop now</Button>}
          />
        </Card>
      </Section>
    </main>
  );
}
