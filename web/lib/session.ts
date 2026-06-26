import "server-only";

import { cookies } from "next/headers";
import { gatewayFetch, SESSION_COOKIE } from "./gateway";

type LoginResponse = {
  access_token: string;
  refresh_token: string;
  expires_at: number; // unix seconds (access token expiry)
};

export async function login(email: string, password: string): Promise<LoginResponse> {
  return gatewayFetch<LoginResponse>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export async function register(email: string, password: string, fullName: string): Promise<{ user_id: string }> {
  return gatewayFetch<{ user_id: string }>("/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, password, full_name: fullName }),
  });
}

// setSession writes the JWT into an httpOnly cookie. The flags matter:
//   httpOnly  -> browser JS can't read it (XSS can't steal the token)
//   sameSite  -> 'lax' blocks the cookie on cross-site POSTs (CSRF mitigation)
//   secure    -> HTTPS-only in production (dev is http://localhost)
//   expires   -> match the token's own lifetime so a dead token isn't kept
// Cookies can only be SET inside a Server Action or Route Handler, never during a
// Server Component render — which is exactly where this is called from.
export async function setSession(token: string, expiresAtUnix: number): Promise<void> {
  const cookieStore = await cookies();
  cookieStore.set({
    name: SESSION_COOKIE,
    value: token,
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    expires: new Date(expiresAtUnix * 1000),
  });
}

export async function clearSession(): Promise<void> {
  (await cookies()).delete(SESSION_COOKIE);
}

// getMe calls the protected /me route; gatewayFetch forwards the cookie as a
// Bearer token. A 401 here means the token is missing/expired/invalid.
export async function getMe(): Promise<{ user_id: string; role: string }> {
  return gatewayFetch<{ user_id: string; role: string }>("/me");
}

// isLoggedIn is a cheap presence check for UI (which nav links to show). It does
// NOT prove the token is valid — that's the gateway's job on each real request.
export async function isLoggedIn(): Promise<boolean> {
  return Boolean((await cookies()).get(SESSION_COOKIE)?.value);
}

// currentRole returns the caller's role, or null when not logged in or the token
// is invalid/expired. Unlike getMe it never throws — it's for UI decisions (e.g.
// whether to show the Admin link), so a 401 just means "no role", not an error.
export async function currentRole(): Promise<string | null> {
  if (!(await isLoggedIn())) return null;
  try {
    return (await getMe()).role;
  } catch {
    return null;
  }
}
