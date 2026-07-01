// SITE_URL is the canonical public origin, used for metadataBase, canonical URLs,
// robots, and the sitemap. Configurable per environment; the trailing slash is
// stripped so callers can safely template `${SITE_URL}${path}`. A malformed value
// falls back to the default rather than crashing every route via new URL().
const DEFAULT_SITE_URL = "https://plateau.example";

function resolveSiteURL(): string {
  const raw = (process.env.NEXT_PUBLIC_SITE_URL ?? DEFAULT_SITE_URL).replace(/\/+$/, "");
  try {
    new URL(raw); // validate it's an absolute URL (metadataBase requires one)
    return raw;
  } catch {
    return DEFAULT_SITE_URL;
  }
}

export const SITE_URL = resolveSiteURL();
