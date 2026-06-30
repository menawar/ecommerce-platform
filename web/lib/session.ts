import "server-only";

import { cookies } from "next/headers";
import { gatewayFetch, SESSION_COOKIE } from "./gateway";
import { SESSION_REFRESH_COOKIE, REFRESH_MAX_AGE_SECONDS } from "./auth-cookies";

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
export async function setSession(token: string, expiresAtUnix: number, refreshToken: string): Promise<void> {
  const cookieStore = await cookies();
  const secure = process.env.NODE_ENV === "production";
  // Access cookie: expires WITH the token, so the browser drops it on expiry —
  // that absence is the signal the middleware uses to know it's time to refresh.
  cookieStore.set({
    name: SESSION_COOKIE,
    value: token,
    httpOnly: true,
    sameSite: "lax",
    secure,
    path: "/",
    expires: new Date(expiresAtUnix * 1000),
  });
  // Refresh cookie: longer-lived (the refresh TTL), used only to mint new access
  // tokens. It outlives the access token so the session survives access expiry.
  cookieStore.set({
    name: SESSION_REFRESH_COOKIE,
    value: refreshToken,
    httpOnly: true,
    sameSite: "lax",
    secure,
    path: "/",
    maxAge: REFRESH_MAX_AGE_SECONDS,
  });
}

// logout revokes the refresh token server-side (so a stolen copy stops working),
// then clears both cookies. Best-effort revoke — the clear is what logs the user
// out locally regardless.
export async function logout(): Promise<void> {
  const refresh = (await cookies()).get(SESSION_REFRESH_COOKIE)?.value;
  if (refresh) {
    try {
      await gatewayFetch("/auth/logout", { method: "POST", body: JSON.stringify({ refresh_token: refresh }) });
    } catch {
      // already invalid / gateway down — nothing to do.
    }
  }
  await clearSession();
}

export async function clearSession(): Promise<void> {
  const cookieStore = await cookies();
  cookieStore.delete(SESSION_COOKIE);
  cookieStore.delete(SESSION_REFRESH_COOKIE);
}

// getMe calls the protected /me route; gatewayFetch forwards the cookie as a
// Bearer token. A 401 here means the token is missing/expired/invalid.
export async function getMe(): Promise<{ user_id: string; role: string; email_verified: boolean }> {
  return gatewayFetch<{ user_id: string; role: string; email_verified: boolean }>("/me");
}

// verifyEmail consumes the single-use token from a verification link. It is a
// public route — no session needed — so an anonymous visitor clicking the link
// can verify. A GatewayError 400 means the token is invalid or expired.
export async function verifyEmail(token: string): Promise<void> {
  await gatewayFetch<void>("/auth/verify-email", {
    method: "POST",
    body: JSON.stringify({ token }),
  });
}

// resendVerification asks for a fresh verification link for the logged-in user.
// The gateway derives the user from the session cookie, so no body is needed.
export async function resendVerification(): Promise<void> {
  await gatewayFetch<void>("/auth/resend-verification", { method: "POST" });
}

// requestPasswordReset asks for a reset link. The gateway always returns 204 (even
// for an unknown email), so this resolves regardless of whether the account exists.
export async function requestPasswordReset(email: string): Promise<void> {
  await gatewayFetch<void>("/auth/forgot-password", {
    method: "POST",
    body: JSON.stringify({ email }),
  });
}

// resetPassword consumes a reset token and sets a new password. A GatewayError 400
// means the token is invalid or expired.
export async function resetPassword(token: string, password: string): Promise<void> {
  await gatewayFetch<void>("/auth/reset-password", {
    method: "POST",
    body: JSON.stringify({ token, password }),
  });
}

// isLoggedIn is a cheap presence check for UI (which nav links to show). It does
// NOT prove the token is valid — that's the gateway's job on each real request.
export async function isLoggedIn(): Promise<boolean> {
  return Boolean((await cookies()).get(SESSION_COOKIE)?.value);
}
