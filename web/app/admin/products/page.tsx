import Link from "next/link";
import { redirect } from "next/navigation";

import { listProducts, GatewayError } from "@/lib/gateway";
import { getMe } from "@/lib/session";
import { formatPrice } from "@/lib/format";
import { ErrorPanel } from "../../error-panel";
import { ProductForm } from "./product-form";
import { DeleteProductButton } from "./delete-button";

const PAGE_SIZE = 100;

// Admin catalog management. A Server Component that gates on the caller's role
// before rendering anything: not logged in -> login; logged in but not admin ->
// a clean "Admins only" panel (the gateway also enforces this, but checking here
// avoids rendering a form a non-admin can't use).
export default async function AdminProductsPage() {
  let role: string;
  try {
    role = (await getMe()).role;
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err;
  }

  if (role !== "admin") {
    return (
      <main style={{ maxWidth: 720, margin: "0 auto", padding: "32px 20px" }}>
        <ErrorPanel message="Admins only — you don't have access to this page." />
      </main>
    );
  }

  let products, total;
  try {
    ({ products, total } = await listProducts({ pageSize: PAGE_SIZE }));
  } catch (err) {
    if (err instanceof GatewayError) {
      return (
        <main style={{ maxWidth: 1000, margin: "0 auto", padding: "32px 20px" }}>
          <h1 style={{ fontSize: 22, fontWeight: 800 }}>Admin · Products</h1>
          <div style={{ marginTop: 20 }}>
            <ErrorPanel message={`Couldn't load products: ${err.message}`} requestId={err.requestId} />
          </div>
        </main>
      );
    }
    throw err;
  }

  return (
    <main style={{ maxWidth: 1000, margin: "0 auto", padding: "32px 20px 60px" }}>
      <h1 style={{ fontSize: 22, fontWeight: 800, marginBottom: 24 }}>Admin · Products</h1>

      <div style={{ display: "flex", gap: 40, flexWrap: "wrap", alignItems: "flex-start" }}>
        {/* Create form */}
        <section style={{ flex: "0 0 460px" }}>
          <h2 style={{ fontSize: 16, fontWeight: 800, marginBottom: 14 }}>New product</h2>
          <ProductForm />
        </section>

        {/* Existing catalog */}
        <section style={{ flex: 1, minWidth: 320 }}>
          <h2 style={{ fontSize: 16, fontWeight: 800, marginBottom: 14 }}>
            Catalog ({total})
          </h2>
          {total > PAGE_SIZE && (
            <p style={{ fontSize: 12, color: "var(--plt-text-secondary)", marginBottom: 10 }}>
              Showing the first {PAGE_SIZE}. Full admin pagination lands with edit/delete.
            </p>
          )}
          {products.length === 0 ? (
            <p style={{ fontSize: 14, color: "var(--plt-text-secondary)" }}>
              No products yet — create the first one.
            </p>
          ) : (
            <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
              <thead>
                <tr style={{ textAlign: "left", color: "var(--plt-text-secondary)" }}>
                  <th style={{ padding: "8px 6px" }}>Name</th>
                  <th style={{ padding: "8px 6px" }}>SKU</th>
                  <th style={{ padding: "8px 6px", textAlign: "right" }}>Price</th>
                  <th style={{ padding: "8px 6px", textAlign: "right" }}>Stock</th>
                  <th style={{ padding: "8px 6px", textAlign: "right" }}></th>
                </tr>
              </thead>
              <tbody>
                {products.map((p) => (
                  <tr key={p.id} style={{ borderTop: "1px solid var(--plt-border)" }}>
                    <td style={{ padding: "8px 6px", fontWeight: 600 }}>{p.name}</td>
                    <td style={{ padding: "8px 6px", fontFamily: "monospace" }}>{p.sku}</td>
                    <td style={{ padding: "8px 6px", textAlign: "right" }}>
                      {formatPrice(p.price_cents, p.currency)}
                    </td>
                    <td style={{ padding: "8px 6px", textAlign: "right" }}>{p.available}</td>
                    <td style={{ padding: "8px 6px", textAlign: "right" }}>
                      <span style={{ display: "inline-flex", gap: 14, alignItems: "center" }}>
                        <Link
                          href={`/admin/products/${p.id}/edit`}
                          style={{ color: "var(--plt-terracotta)", textDecoration: "none", fontWeight: 600 }}
                        >
                          Edit
                        </Link>
                        <DeleteProductButton id={p.id} name={p.name} />
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      </div>
    </main>
  );
}
