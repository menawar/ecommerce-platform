"use server";

// "use server" marks every export here as a Server Action: code that runs ONLY on
// the server but can be invoked from a form/client. The framework creates a secure
// RPC endpoint for each — the client calls it, the body executes server-side.
import { redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import {
  login,
  logout,
  register,
  requestPasswordReset,
  resendVerification,
  resetPassword,
  setSession,
  verifyEmail,
} from "@/lib/session";

// The shape useActionState threads between submissions (prev state -> next state).
export type AuthState = { error?: string };

export async function loginAction(_prev: AuthState, formData: FormData): Promise<AuthState> {
  const email = String(formData.get("email") ?? "");
  const password = String(formData.get("password") ?? "");

  try {
    const { access_token, refresh_token, expires_at } = await login(email, password);
    await setSession(access_token, expires_at, refresh_token);
  } catch (err) {
    if (err instanceof GatewayError) {
      return { error: err.status === 401 ? "Invalid email or password" : err.message };
    }
    throw err;
  }
  // redirect() throws a special signal Next handles as navigation. It MUST be
  // outside the try/catch — otherwise the catch would swallow the redirect.
  redirect("/products");
}

// A no-arg Server Action: a <form action={logoutAction}> needs no client JS and no
// useActionState — submit clears the cookie and redirects.
export async function logoutAction(): Promise<void> {
  await logout(); // revoke the refresh token server-side, then clear cookies
  redirect("/login");
}

// VerifyState threads the result of a verify-email submission back to the form.
// Verification runs ONLY from this action (a POST on explicit click), never on a
// GET page render — so email link-scanners/prefetchers can't silently consume the
// single-use token before the human acts.
export type VerifyState = { status?: "ok" | "invalid" };

export async function verifyEmailAction(_prev: VerifyState, formData: FormData): Promise<VerifyState> {
  const token = String(formData.get("token") ?? "");
  if (!token) return { status: "invalid" };
  try {
    await verifyEmail(token);
  } catch (err) {
    // 400 is the expected "invalid or expired" answer; anything else is a real
    // fault and should reach the error boundary.
    if (err instanceof GatewayError && err.status === 400) return { status: "invalid" };
    throw err;
  }
  return { status: "ok" };
}

// ResendState threads the outcome of a resend-verification submission back to the
// form so it can confirm "sent" or surface an error.
export type ResendState = { sent?: boolean; error?: string };

export async function resendVerificationAction(): Promise<ResendState> {
  try {
    await resendVerification();
  } catch (err) {
    if (err instanceof GatewayError) {
      // 401 means the session lapsed — the link page will prompt a re-login.
      return { error: err.status === 401 ? "Please sign in to resend the link." : err.message };
    }
    throw err;
  }
  return { sent: true };
}

// ForgotState threads the outcome of a forgot-password submission. On success we
// show the SAME confirmation whether or not the email exists (enumeration-safe);
// only a genuine infrastructure failure surfaces an error.
export type ForgotState = { sent?: boolean; error?: string };

export async function forgotPasswordAction(_prev: ForgotState, formData: FormData): Promise<ForgotState> {
  const email = String(formData.get("email") ?? "");
  try {
    await requestPasswordReset(email);
  } catch (err) {
    if (err instanceof GatewayError) {
      return { error: "Something went wrong — please try again." };
    }
    throw err;
  }
  return { sent: true };
}

// ResetPwState threads a reset-password failure back to the form. Success
// redirects to login, so there's no success state to render.
export type ResetPwState = { error?: string };

export async function resetPasswordAction(_prev: ResetPwState, formData: FormData): Promise<ResetPwState> {
  const token = String(formData.get("token") ?? "");
  const password = String(formData.get("password") ?? "");
  try {
    await resetPassword(token, password);
  } catch (err) {
    if (err instanceof GatewayError) {
      // Password length is validated in the form (minLength), so a 400 here means
      // the token is bad/expired. Other statuses get a generic message rather than
      // leaking the raw gateway/gRPC text.
      return {
        error:
          err.status === 400
            ? "This reset link is invalid or has expired."
            : "Something went wrong — please try again.",
      };
    }
    throw err;
  }
  // Sessions were revoked server-side; send the user to log in with the new password.
  redirect("/login?reset=1");
}

export async function registerAction(_prev: AuthState, formData: FormData): Promise<AuthState> {
  const email = String(formData.get("email") ?? "");
  const password = String(formData.get("password") ?? "");
  const fullName = String(formData.get("full_name") ?? "");

  try {
    await register(email, password, fullName);
    // Auto-login after a successful registration for a smoother first run.
    const { access_token, refresh_token, expires_at } = await login(email, password);
    await setSession(access_token, expires_at, refresh_token);
  } catch (err) {
    if (err instanceof GatewayError) {
      if (err.status === 409) return { error: "That email is already registered" };
      if (err.status === 400) return { error: err.message }; // e.g. password too short
      return { error: err.message };
    }
    throw err;
  }
  redirect("/products");
}
