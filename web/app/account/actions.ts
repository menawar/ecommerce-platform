"use server";

import { redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { clearSession } from "@/lib/session";
import { deleteMyAccount } from "@/lib/data-export";

// deleteAccountAction erases the account, clears the session cookie, and sends the
// (now anonymous) visitor home. A 401 mid-flight also clears + bounces to login.
export async function deleteAccountAction(): Promise<void> {
  try {
    await deleteMyAccount();
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) {
      await clearSession();
      redirect("/login");
    }
    throw err;
  }
  await clearSession();
  redirect("/?deleted=1");
}
