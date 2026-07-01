import type { MetadataRoute } from "next";
import { SITE_URL } from "@/lib/site";
import { listProducts } from "@/lib/gateway";

// The public, indexable routes plus a page per product. If the gateway is
// unreachable at generation time we still emit a valid sitemap of the static
// routes rather than failing the whole file.
export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const staticRoutes: MetadataRoute.Sitemap = ["", "/products", "/terms", "/privacy"].map((path) => ({
    url: `${SITE_URL}${path}`,
    lastModified: new Date(),
  }));

  const products: MetadataRoute.Sitemap = [];
  try {
    // Page through the whole catalog so products past the first page still get indexed.
    const pageSize = 100;
    for (let page = 1; ; page++) {
      const { products: items } = await listProducts({ page, pageSize });
      for (const p of items) {
        products.push({ url: `${SITE_URL}/products/${p.id}`, lastModified: new Date(p.created_at * 1000) });
      }
      if (items.length < pageSize) break;
    }
  } catch {
    // Gateway down at generation time — the static routes above still make a valid sitemap.
  }

  return [...staticRoutes, ...products];
}
