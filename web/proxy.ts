import { NextResponse, type NextRequest } from "next/server";

// In Next 16 the old `middleware.ts` convention is renamed to `proxy.ts` — same
// idea: code that runs at the edge before a request reaches a route.
//
// This is a COARSE, fast gate: if there's no session cookie, don't even render a
// protected page — bounce to /login. It is NOT the security boundary. A present-
// but-expired cookie passes here; real authorization happens when the page calls
// the gateway, which validates the JWT via the User service. Never trust the edge
// gate alone — it can't verify the token's signature (and shouldn't hold the secret).
const SESSION_COOKIE = "session";

export function proxy(request: NextRequest) {
  if (!request.cookies.has(SESSION_COOKIE)) {
    const loginUrl = new URL("/login", request.url);
    loginUrl.searchParams.set("next", request.nextUrl.pathname);
    return NextResponse.redirect(loginUrl);
  }
  return NextResponse.next();
}

// matcher scopes the proxy to protected routes only — catalog/auth stay public,
// and static assets are never touched.
export const config = {
  matcher: ["/account/:path*", "/cart"],
};
