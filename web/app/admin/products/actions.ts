"use server";

import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";

import { createProduct, GatewayError } from "@/lib/gateway";
import { getMe } from "@/lib/session";
import { parsePriceToCents } from "@/lib/money";
import { uploadImage, deleteImage, UploadError } from "@/lib/storage";

export type CreateProductState = { error: string } | null;

// createProductAction handles the admin create-product form. It validates input,
// converts the price to integer cents, and POSTs to the gateway. The gateway is
// the real authorization boundary (requireAdmin); the getMe check here just avoids
// a pointless round-trip and gives a non-admin a clear message instead of a raw
// 403. Field validation mirrors the Product service so the user sees errors inline
// rather than as opaque gateway 400s.
export async function createProductAction(
  _prev: CreateProductState,
  formData: FormData,
): Promise<CreateProductState> {
  try {
    const me = await getMe();
    if (me.role !== "admin") return { error: "Admins only." };
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err;
  }

  const sku = String(formData.get("sku") ?? "").trim();
  const name = String(formData.get("name") ?? "").trim();
  const description = String(formData.get("description") ?? "").trim();
  const currency = String(formData.get("currency") ?? "").trim() || "NGN";
  const priceCents = parsePriceToCents(String(formData.get("price") ?? ""));
  const quantity = Number(formData.get("initial_quantity") ?? 0);

  if (!sku || !name) return { error: "SKU and name are required." };
  if (priceCents === null) return { error: "Enter a valid, non-negative price." };
  if (!Number.isInteger(quantity) || quantity < 0) {
    return { error: "Enter a valid, non-negative stock quantity." };
  }

  // Optional image: upload to object storage and store the resulting public URL.
  // A 0-byte entry means "no file chosen" (browsers submit an empty File then).
  let imageURL = "";
  const file = formData.get("image");
  if (file instanceof File && file.size > 0) {
    try {
      imageURL = await uploadImage(file);
    } catch (err) {
      if (err instanceof UploadError) return { error: err.message };
      throw err;
    }
  }

  try {
    await createProduct({
      sku,
      name,
      description,
      currency,
      image_url: imageURL,
      price_cents: priceCents,
      category_id: "", // uncategorized for now (no category management yet)
      initial_quantity: quantity,
    });
  } catch (err) {
    // The product wasn't created, so a just-uploaded image is now orphaned in the
    // bucket — best-effort remove it. Cleanup failure must never mask the real
    // error, so it's swallowed.
    if (imageURL) {
      try {
        await deleteImage(imageURL);
      } catch {
        // ignore — orphan cleanup is best-effort
      }
    }
    if (err instanceof GatewayError) {
      if (err.status === 401) redirect("/login");
      if (err.status === 409) return { error: "A product with that SKU already exists." };
      return { error: err.message };
    }
    throw err;
  }

  // The new product must show up immediately in both the admin list and the
  // public storefront, so drop their cached renders before redirecting back.
  revalidatePath("/admin/products");
  revalidatePath("/products");
  redirect("/admin/products");
}
