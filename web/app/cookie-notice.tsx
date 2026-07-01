"use client";

import { useSyncExternalStore } from "react";
import Link from "next/link";

const STORAGE_KEY = "plt-cookie-ack";

// The acknowledgement lives in localStorage — an external store — so we read it with
// useSyncExternalStore rather than a setState-in-effect. getServerSnapshot returns
// "acknowledged" so nothing renders during SSR (no flash / hydration mismatch); the
// real value is read on the client after mount.
const ackListeners = new Set<() => void>();

function subscribe(cb: () => void) {
  ackListeners.add(cb);
  return () => ackListeners.delete(cb);
}

function readAck(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === "1";
  } catch {
    return false; // localStorage unavailable (private mode) — show the notice
  }
}

function acknowledge() {
  try {
    localStorage.setItem(STORAGE_KEY, "1");
  } catch {
    // ignore write failures
  }
  ackListeners.forEach((l) => l());
}

// CookieNotice shows a one-time, dismissible notice that we use a strictly-necessary
// session cookie. Because that cookie is required for the site to work (no tracking
// or ad cookies), this is a notice — not a consent gate.
export function CookieNotice() {
  const acked = useSyncExternalStore(subscribe, readAck, () => true);
  if (acked) return null;

  return (
    <div
      role="region"
      aria-label="Cookie notice"
      style={{
        position: "fixed",
        bottom: 16,
        left: 16,
        right: 16,
        maxWidth: 560,
        margin: "0 auto",
        background: "var(--plt-green-deep)",
        color: "#fff",
        borderRadius: 12,
        padding: "14px 16px",
        display: "flex",
        alignItems: "center",
        gap: 14,
        boxShadow: "0 8px 30px rgba(0,0,0,0.25)",
        zIndex: 50,
        fontSize: 13.5,
        lineHeight: 1.5,
      }}
    >
      <span style={{ flex: 1 }}>
        We use a single strictly-necessary cookie to keep you signed in — no tracking or ads. See our{" "}
        <Link href="/privacy" style={{ color: "#f3b73f", textDecoration: "underline" }}>
          Privacy Policy
        </Link>
        .
      </span>
      <button onClick={acknowledge} className="plt-btn-gold" style={{ whiteSpace: "nowrap" }}>
        Got it
      </button>
    </div>
  );
}
