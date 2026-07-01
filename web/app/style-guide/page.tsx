import type { Metadata } from "next";
import { cn } from "@/lib/cn";

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
    </main>
  );
}
