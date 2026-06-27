import type { NextConfig } from "next";

// securityHeaders harden every browser-facing response. They live here (not on the
// gateway) because in this BFF architecture the browser only ever talks to Next.js
// — the gateway is server-to-server behind it.
//
// The CSP is a pragmatic baseline: 'unsafe-inline' is required for now because the
// app uses inline style props and Next's hydration emits inline scripts (moving to
// nonce-based script-src is a future hardening step). It still meaningfully locks
// down framing, object/base-uri, and the default fetch origin.
const csp = [
  "default-src 'self'",
  "img-src 'self' data: blob: https: http://localhost:9000", // product images: dev MinIO + prod CDN/S3 over https
  "style-src 'self' 'unsafe-inline'",
  "script-src 'self' 'unsafe-inline'",
  "font-src 'self' data:",
  "connect-src 'self'",
  "frame-ancestors 'none'", // clickjacking: the app may not be embedded
  "base-uri 'self'",
  "form-action 'self'",
  "object-src 'none'",
].join("; ");

const securityHeaders = [
  { key: "Content-Security-Policy", value: csp },
  // Browsers honour HSTS only over HTTPS, so it's a safe no-op in local http dev
  // and forces TLS in production.
  { key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains; preload" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(), browsing-topics=()" },
];

const nextConfig: NextConfig = {
  experimental: {
    serverActions: {
      // Product-image uploads go through a Server Action, which defaults to a 1 MB
      // request-body cap. Lift it just above our 5 MB image limit (lib/upload-validate
      // enforces the real size check) so a normal photo isn't rejected at the edge.
      bodySizeLimit: "6mb",
    },
  },
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
};

export default nextConfig;
