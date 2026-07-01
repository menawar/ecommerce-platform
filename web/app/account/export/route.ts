import { redirect } from "next/navigation";

import { GatewayError } from "@/lib/gateway";
import { exportMyData } from "@/lib/data-export";

// GET /account/export streams the user's data as a downloadable JSON file. The BFF
// calls the gateway server-to-server (JWT from the httpOnly cookie) and re-emits the
// body with a Content-Disposition so the browser saves it. Reached via an <a>.
export async function GET() {
  let data;
  try {
    data = await exportMyData();
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    // Reached via a full-page <a> navigation, so there's no error boundary — return a
    // readable message instead of a raw 500 stack.
    if (err instanceof GatewayError) {
      return new Response("Couldn't export your data right now — please try again shortly.", {
        status: 502,
        headers: { "content-type": "text/plain; charset=utf-8" },
      });
    }
    throw err;
  }
  return new Response(JSON.stringify(data, null, 2), {
    headers: {
      "content-type": "application/json; charset=utf-8",
      "content-disposition": 'attachment; filename="plateau-data-export.json"',
    },
  });
}
