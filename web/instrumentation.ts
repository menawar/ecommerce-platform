import { reportError } from "@/lib/report-error";

// Next's server-side error hook: every uncaught error in a Server Component, route
// handler, or Server Action lands here. We forward it to the reporting seam with a
// little request context. (Client errors are captured by app/global-error.tsx.)
export function onRequestError(
  err: unknown,
  request: { path?: string; method?: string },
): void {
  reportError(err, { path: request.path, method: request.method });
}
