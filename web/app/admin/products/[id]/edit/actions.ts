"use server";

import { redirect } from "next/navigation";
import { revalidatePath } from "next/cache";

import { updateProduct, GatewayError, type UpdateProductInput } from "@/lib/gateway";
import { getMe } from "@/lib/session";
import { parsePriceToCents } from "@/lib/money";
import { uploadImage, deleteImage, UploadError } from "@/lib/storage";

export type EditProductState = { error: string } | null;

// updateProductAction handles the admin edit form. Catalog fields are a full
// replace; stock is only changed when a value is entered (blank = leave as-is, so
// editing a name never disturbs inventory). An optional new image is uploaded and
// the replaced one is best-effort deleted to avoid orphans.
export async function updateProductAction(
  _prev: EditProductState,
  formData: FormData,
): Promise<EditProductState> {
  try {
    const me = await getMe();
    if (me.role !== "admin") return { error: "Admins only." };
  } catch (err) {
    if (err instanceof GatewayError && err.status === 401) redirect("/login");
    throw err;
  }

  const id = String(formData.get("id") ?? "").trim();
  const name = String(formData.get("name") ?? "").trim();
  const description = String(formData.get("description") ?? "").trim();
  const currency = String(formData.get("currency") ?? "").trim() || "NGN";
  const currentImage = String(formData.get("current_image_url") ?? "");
  const categoryID = String(formData.get("category_id") ?? ""); // preserved, not edited here
  const priceCents = parsePriceToCents(String(formData.get("price") ?? ""));

  if (!id) return { error: "Missing product id." };
  if (!name) return { error: "Name is required." };
  if (priceCents === null) return { error: "Enter a valid, non-negative price." };

  // Stock is optional: blank means "leave unchanged"; a value must be a
  // non-negative integer and is sent as an absolute level.
  const stockRaw = String(formData.get("quantity") ?? "").trim();
  let quantity: number | undefined;
  if (stockRaw !== "") {
    const n = Number(stockRaw);
    if (!Number.isInteger(n) || n < 0) {
      return { error: "Stock must be a whole number of 0 or more." };
    }
    quantity = n;
  }

  // Optional image replacement.
  let imageURL = currentImage;
  let replacedOld = false;
  const file = formData.get("image");
  if (file instanceof File && file.size > 0) {
    try {
      imageURL = await uploadImage(file);
      replacedOld = currentImage !== "" && currentImage !== imageURL;
    } catch (err) {
      if (err instanceof UploadError) return { error: err.message };
      throw err;
    }
  }

  const input: UpdateProductInput = {
    name,
    description,
    currency,
    category_id: categoryID,
    image_url: imageURL,
    price_cents: priceCents,
    ...(quantity !== undefined ? { quantity } : {}),
  };

  try {
    await updateProduct(id, input);
  } catch (err) {
    // Update failed — drop a just-uploaded image so it isn't orphaned.
    if (imageURL !== currentImage) {
      try {
        await deleteImage(imageURL);
      } catch {
        // best-effort
      }
    }
    if (err instanceof GatewayError) {
      if (err.status === 401) redirect("/login");
      if (err.status === 404) return { error: "Product not found." };
      return { error: err.message }; // e.g. 422 "stock below reserved units"
    }
    throw err;
  }

  // Success — if a new image replaced an old one, best-effort delete the old.
  if (replacedOld) {
    try {
      await deleteImage(currentImage);
    } catch {
      // best-effort
    }
  }

  revalidatePath("/admin/products");
  revalidatePath("/products");
  redirect("/admin/products");
}
