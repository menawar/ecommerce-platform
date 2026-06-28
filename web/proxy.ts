import { NextResponse, type NextRequest } from "next/server";

import {
  SESSION_COOKIE,
  SESSION_REFRESH_COOKIE,
  REFRESH_MAX_AGE_SECONDS,
} from "@/lib/auth-cookies";

// In Next 16 the old `middleware.ts` convention is `proxy.ts` — code that runs at
// the edge before a request reaches a route. It does two jobs:
//
//  1. TRANSPARENT REFRESH. The access cookie expires WITH the access token, so the
//     browser drops it; "no access cookie but a refresh cookie present" means it
//     just expired. We exchange the refresh token for a fresh pair here — the one
//     place that can WRITE cookies outside a Server Action — so a 15-minute access
//     token renews invisibly and the user stays logged in for the refresh TTL.
//
//  2. A COARSE GATE. Visiting a protected route with no (valid) session bounces to
//     /login. This is NOT the security boundary: a present-but-expired cookie is
//     not verified here — real authorization happens when the page calls the
//     gateway, which validates the JWT via the User service.
const GATEWAY_URL = process.env.GATEWAY_URL ?? "http://localhost:8080";

const PROTECTED = [/^\/account(\/|$)/, /^\/cart$/, /^\/checkout/, /^\/orders(\/|$)/];
const isProtected = (path: string) => PROTECTED.some((re) => re.test(path));

type Tokens = { access_token: string; refresh_token: string; expires_at: number };

function setAuthCookies(res: NextResponse, t: Tokens) {
  const secure = process.env.NODE_ENV === "production";
  res.cookies.set(SESSION_COOKIE, t.access_token, {
    httpOnly: true,
    sameSite: "lax",
    secure,
    path: "/",
    expires: new Date(t.expires_at * 1000),
  });
  res.cookies.set(SESSION_REFRESH_COOKIE, t.refresh_token, {
    httpOnly: true,
    sameSite: "lax",
    secure,
    path: "/",
    maxAge: REFRESH_MAX_AGE_SECONDS,
  });
}

export async function proxy(request: NextRequest) {
  let access = request.cookies.get(SESSION_COOKIE)?.value;
  const refresh = request.cookies.get(SESSION_REFRESH_COOKIE)?.value;
  let refreshed: Tokens | null = null;

  // 1. Transparent refresh: access expired but a refresh token remains.
  if (!access && refresh) {
    try {
      const r = await fetch(`${GATEWAY_URL}/auth/refresh`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ refresh_token: refresh }),
        cache: "no-store",
      });
      if (r.ok) {
        refreshed = (await r.json()) as Tokens;
        access = refreshed.access_token;
        request.cookies.set(SESSION_COOKIE, access); // so THIS render sees it
      }
    } catch {
      // gateway unreachable — fall through and treat as logged out
    }
  }

  const deadRefresh = !access && Boolean(refresh); // had a refresh token, it failed

  // 2. Gate: a protected route with no usable session → /login.
  if (isProtected(request.nextUrl.pathname) && !access) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("next", request.nextUrl.pathname);
    const res = NextResponse.redirect(loginUrl);
    if (deadRefresh) {
      res.cookies.delete(SESSION_COOKIE);
      res.cookies.delete(SESSION_REFRESH_COOKIE);
    }
    return res;
  }

  // 3. Proceed — persisting a refreshed pair, or clearing a dead session.
  const res = NextResponse.next({ request: { headers: request.headers } });
  if (refreshed) {
    setAuthCookies(res, refreshed);
  } else if (deadRefresh) {
    res.cookies.delete(SESSION_COOKIE);
    res.cookies.delete(SESSION_REFRESH_COOKIE);
  }
  return res;
}

// Run on app routes (so refresh works everywhere, keeping nav state honest); skip
// Next internals + static assets.
export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
