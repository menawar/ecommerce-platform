import Link from "next/link";
import { redirect, notFound } from "next/navigation";

import { getProduct, GatewayError } from "@/lib/gateway";
import { getMe } from "@/lib/session";
import { ErrorPanel } from "../../../../error-panel";
import { EditForm } from "./edit-form";

// Admin edit page for a single product. Gates on role, then fetches the current
// product to pre-fill the form. A 404 from the gateway becomes Next's notFound().
export default async function EditProductPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

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

  let product;
  try {
    product = await getProduct(id);
  } catch (err) {
    if (err instanceof GatewayError) {
      if (err.status === 404) notFound();
      return (
        <main style={{ maxWidth: 720, margin: "0 auto", padding: "32px 20px" }}>
          <ErrorPanel message={`Couldn't load product: ${err.message}`} requestId={err.requestId} />
        </main>
      );
    }
    throw err;
  }

  return (
    <main style={{ maxWidth: 720, margin: "0 auto", padding: "32px 20px 60px" }}>
      <div style={{ marginBottom: 20, fontSize: 13 }}>
        <Link href="/admin/products" style={{ color: "var(--plt-text-secondary)", textDecoration: "none" }}>
          ← Back to products
        </Link>
      </div>
      <h1 style={{ fontSize: 22, fontWeight: 800, marginBottom: 24 }}>Edit · {product.name}</h1>
      <EditForm product={product} />
    </main>
  );
}
