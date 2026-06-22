// The typed, server-side gateway client. `server-only` makes importing this from
// a Client Component a BUILD error — a guardrail so the JWT-reading code can never
// be bundled into the browser.
import "server-only";

import { cookies } from "next/headers";
import type { Product, ProductList } from "./types";

const GATEWAY_URL = process.env.GATEWAY_URL ?? "http://localhost:8080";

// The httpOnly cookie name the login Server Action will set (Step: auth).
export const SESSION_COOKIE = "session";

// GatewayError carries the HTTP status so callers can branch — e.g. a detail page
// turns a 404 into Next's notFound(), everything else into the error boundary.
export class GatewayError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "GatewayError";
  }
}

// gatewayFetch is the one place every gateway call goes through. It runs ONLY on
// the server (it reads the httpOnly cookie via next/headers), attaches the JWT as
// a Bearer token if present, and normalizes the gateway's {"error": "..."} body
// into a typed GatewayError. The browser never sees the token or the gateway URL.
export async function gatewayFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = (await cookies()).get(SESSION_COOKIE)?.value;

  let res: Response;
  try {
    res = await fetch(`${GATEWAY_URL}${path}`, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...init.headers,
      },
      // Reads should reflect current data, not a cached snapshot. (Next 16 doesn't
      // cache fetch by default, but we're explicit about intent.)
      cache: "no-store",
    });
  } catch {
    // fetch() REJECTS (vs. returning a non-2xx) only on network-level failures:
    // connection refused, DNS, timeout — i.e. the gateway isn't reachable. The raw
    // "fetch failed" is useless to a user, so surface an actionable message instead.
    throw new GatewayError(
      503,
      `Cannot reach the API gateway at ${GATEWAY_URL}. Make sure the backend services are running.`,
    );
  }

  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = (await res.json()) as { error?: string };
      if (body?.error) message = body.error;
    } catch {
      // non-JSON error body — fall back to statusText
    }
    throw new GatewayError(res.status, message);
  }

  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export async function listProducts(params: {
  page?: number;
  pageSize?: number;
  q?: string;
  categoryId?: string;
}): Promise<ProductList> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.pageSize) qs.set("page_size", String(params.pageSize));
  if (params.q) qs.set("q", params.q);
  if (params.categoryId) qs.set("category_id", params.categoryId);
  const suffix = qs.toString() ? `?${qs}` : "";
  return gatewayFetch<ProductList>(`/products${suffix}`);
}

export async function getProduct(id: string): Promise<Product> {
  return gatewayFetch<Product>(`/products/${encodeURIComponent(id)}`);
}
