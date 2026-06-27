"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

// StatusPoller re-fetches this (server) page on an interval while the order is
// awaiting payment, so it flips to CONFIRMED/CANCELLED automatically once the
// payment webhook settles the saga — no manual refresh. It renders nothing.
export function StatusPoller({ intervalMs = 2500 }: { intervalMs?: number }) {
  const router = useRouter();
  useEffect(() => {
    const id = setInterval(() => router.refresh(), intervalMs);
    return () => clearInterval(id);
  }, [router, intervalMs]);
  return null;
}
