"use server";

// "use server" marks every export here as a Server Action: code that runs ONLY on
// the server but can be invoked from a form/client. The framework creates a secure
// RPC endpoint for each — the client calls it, the body executes server-side.
import { redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { login, register, setSession } from "@/lib/session";

// The shape useActionState threads between submissions (prev state -> next state).
export type AuthState = { error?: string };

export async function loginAction(_prev: AuthState, formData: FormData): Promise<AuthState> {
  const email = String(formData.get("email") ?? "");
  const password = String(formData.get("password") ?? "");

  try {
    const { access_token, expires_at } = await login(email, password);
    await setSession(access_token, expires_at);
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

export async function registerAction(_prev: AuthState, formData: FormData): Promise<AuthState> {
  const email = String(formData.get("email") ?? "");
  const password = String(formData.get("password") ?? "");
  const fullName = String(formData.get("full_name") ?? "");

  try {
    await register(email, password, fullName);
    // Auto-login after a successful registration for a smoother first run.
    const { access_token, expires_at } = await login(email, password);
    await setSession(access_token, expires_at);
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
