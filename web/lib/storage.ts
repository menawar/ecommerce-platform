import "server-only";

import { randomUUID } from "node:crypto";

import { S3Client, PutObjectCommand, DeleteObjectCommand } from "@aws-sdk/client-s3";

import { imageObjectKey, isAllowedImageType, MAX_IMAGE_BYTES } from "./upload-validate";

// Two distinct endpoints by design:
//   S3_ENDPOINT        — how THIS (the Next server) reaches the bucket to upload.
//   S3_PUBLIC_BASE_URL — how the BROWSER reaches the bucket to load the <img>.
// They differ in containerized setups (server talks to "minio:9000", browser to
// "localhost:9000"), so keeping them separate avoids "uploads fine, 404s in the
// browser" bugs. forcePathStyle is required for MinIO (and harmless for S3/R2).
const ENDPOINT = process.env.S3_ENDPOINT ?? "http://localhost:9000";
const PUBLIC_BASE_URL = process.env.S3_PUBLIC_BASE_URL ?? "http://localhost:9000/product-images";
const BUCKET = process.env.S3_BUCKET ?? "product-images";
const REGION = process.env.S3_REGION ?? "us-east-1";

const client = new S3Client({
  endpoint: ENDPOINT,
  region: REGION,
  forcePathStyle: true,
  credentials: {
    accessKeyId: process.env.S3_ACCESS_KEY ?? "minioadmin",
    secretAccessKey: process.env.S3_SECRET_KEY ?? "minioadmin",
  },
});

export class UploadError extends Error {}

// uploadImage validates and PUTs a product image, returning its public URL. It
// re-checks type and size here (defense in depth alongside the form/action) so a
// crafted request can't bypass the UI guards. The key is uuid-based and the
// extension comes from the content-type, never the client filename.
export async function uploadImage(file: File): Promise<string> {
  if (!isAllowedImageType(file.type)) {
    throw new UploadError("Unsupported image type — use PNG, JPEG, WebP, or GIF.");
  }
  if (file.size > MAX_IMAGE_BYTES) {
    throw new UploadError("Image is too large — 5 MB max.");
  }

  const key = imageObjectKey(file.type, randomUUID());
  const body = Buffer.from(await file.arrayBuffer());

  await client.send(
    new PutObjectCommand({
      Bucket: BUCKET,
      Key: key,
      Body: body,
      ContentType: file.type,
    }),
  );

  // key already starts with "products/"; PUBLIC_BASE_URL points at the bucket root.
  return `${PUBLIC_BASE_URL.replace(/\/$/, "")}/${key}`;
}

// deleteImage best-effort removes an object previously returned by uploadImage,
// given its public URL. Used to avoid orphaning the upload when the subsequent
// product create fails (e.g. duplicate SKU). Best-effort by design: a failed
// cleanup must not mask the original error, so the caller ignores any throw.
export async function deleteImage(publicUrl: string): Promise<void> {
  const base = PUBLIC_BASE_URL.replace(/\/$/, "");
  if (!publicUrl.startsWith(`${base}/`)) return; // not one of ours — leave it alone
  const key = publicUrl.slice(base.length + 1);
  await client.send(new DeleteObjectCommand({ Bucket: BUCKET, Key: key }));
}
