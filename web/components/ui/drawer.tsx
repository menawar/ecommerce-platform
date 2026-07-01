"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/cn";

// A modal slide-in panel (mobile menu, filters, mini-cart). Controlled via `open` /
// `onClose`. Accessible: role=dialog + aria-modal, Escape + overlay-click to close,
// body-scroll lock, focus moved into the panel and trapped with Tab, and restored to
// the trigger on close. Rendered through a portal to escape stacking/overflow.
const FOCUSABLE =
  'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

export function Drawer({
  open,
  onClose,
  side = "left",
  title,
  children,
  className,
}: {
  open: boolean;
  onClose: () => void;
  side?: "left" | "right";
  title: string;
  children: ReactNode;
  className?: string;
}) {
  const panelRef = useRef<HTMLDivElement>(null);

  // Latest-ref for onClose so the open-effect depends only on `open` — otherwise an
  // inline onClose (new identity each render) would tear down + re-run the effect on
  // every parent render while open, thrashing focus and the scroll lock.
  const onCloseRef = useRef(onClose);
  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    // NOTE: assumes a single drawer open at a time (fine for our usage — one mobile
    // menu / mini-cart). Nested concurrent drawers would need a ref-counted lock.
    if (!open) return;
    const previouslyFocused = document.activeElement as HTMLElement | null;
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    panelRef.current?.focus();

    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onCloseRef.current();
        return;
      }
      if (e.key !== "Tab") return;
      const panel = panelRef.current;
      const nodes = panel?.querySelectorAll<HTMLElement>(FOCUSABLE);
      if (!panel || !nodes || nodes.length === 0) return;
      // If focus has escaped the panel (e.g. after an overlay click), pull it back.
      if (!panel.contains(document.activeElement)) {
        e.preventDefault();
        nodes[0].focus();
        return;
      }
      const first = nodes[0];
      const last = nodes[nodes.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKey);

    return () => {
      document.body.style.overflow = prevOverflow;
      document.removeEventListener("keydown", onKey);
      previouslyFocused?.focus?.();
    };
  }, [open]);

  if (!open || typeof document === "undefined") return null;

  return createPortal(
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} aria-hidden />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        tabIndex={-1}
        className={cn(
          "absolute top-0 flex h-full w-[86%] max-w-sm flex-col bg-card shadow-lg outline-none",
          side === "left" ? "left-0" : "right-0",
          className,
        )}
      >
        <div className="flex items-center justify-between border-b border-border px-4 py-3">
          <span className="font-bold text-fg">{title}</span>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="rounded-md p-1 text-lg leading-none text-fg-muted hover:bg-surface"
          >
            ✕
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-4">{children}</div>
      </div>
    </div>,
    document.body,
  );
}
