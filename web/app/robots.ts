import type { MetadataRoute } from "next";
import { SITE_URL } from "@/lib/site";

// robots.txt: crawlers may index the public storefront but not the authenticated
// or transactional areas (nothing useful to index, and we don't want them crawled).
export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: [
        "/account",
        "/admin",
        "/cart",
        "/checkout",
        "/orders",
        "/login",
        "/register",
        // Token-bearing transactional pages — nothing to index, and their URLs
        // can carry one-time tokens.
        "/forgot-password",
        "/reset-password",
        "/verify-email",
      ],
    },
    sitemap: `${SITE_URL}/sitemap.xml`,
  };
}
