import { gatewayFetch } from "./gateway";

// DataExport mirrors the gateway's /me/export payload — the user's personal data
// (profile, addresses, orders) for the NDPR/GDPR access & portability right.
export type DataExport = {
  exported_at: string;
  profile: {
    user_id: string;
    email: string;
    full_name: string;
    role: string;
    email_verified: boolean;
  };
  addresses: unknown[];
  orders: unknown[];
};

export function exportMyData(): Promise<DataExport> {
  return gatewayFetch<DataExport>("/me/export");
}

// deleteMyAccount triggers erasure (NDPR/GDPR right to erasure). The gateway
// anonymises the account and revokes sessions; the caller then clears cookies.
export async function deleteMyAccount(): Promise<void> {
  await gatewayFetch<void>("/me/delete", { method: "POST" });
}
