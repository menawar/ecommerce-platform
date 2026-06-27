// Pure helpers for the image-upload path — no I/O, no server-only imports, so they
// unit-test trivially. The actual S3 PUT lives in lib/storage.ts.

// Max image size accepted from the admin form. Kept in lockstep with the Next
// server-action body limit (next.config) — the action must accept a body at least
// this large or the upload fails before our own check can run.
export const MAX_IMAGE_BYTES = 5 * 1024 * 1024; // 5 MB

// content-type -> file extension. Also acts as the allow-list: a type that isn't a
// key here is rejected.
const EXT_BY_TYPE: Record<string, string> = {
  "image/png": "png",
  "image/jpeg": "jpg",
  "image/webp": "webp",
  "image/gif": "gif",
};

export function isAllowedImageType(contentType: string): boolean {
  return contentType in EXT_BY_TYPE;
}

// imageObjectKey builds the bucket key for an upload: a random uuid (so two files
// with the same name never collide or overwrite) under a products/ prefix, with
// the extension derived from the trusted content-type — never from the client
// filename, which could carry a misleading or unsafe extension.
export function imageObjectKey(contentType: string, uuid: string): string {
  const ext = EXT_BY_TYPE[contentType];
  if (!ext) throw new Error(`unsupported image type: ${contentType}`);
  return `products/${uuid}.${ext}`;
}
