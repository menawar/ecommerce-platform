import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  experimental: {
    serverActions: {
      // Product-image uploads go through a Server Action, which defaults to a 1 MB
      // request-body cap. Lift it just above our 5 MB image limit (lib/upload-validate
      // enforces the real size check) so a normal photo isn't rejected at the edge.
      bodySizeLimit: "6mb",
    },
  },
};

export default nextConfig;
