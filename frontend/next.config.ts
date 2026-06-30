import type { NextConfig } from "next";
import { dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));

const nextConfig: NextConfig = {
  output: 'standalone',
  // Ensure Next/Turbopack uses this folder as the project root
  // This avoids the multiple lockfiles mis-detection when running from a monorepo
  // See warning: "Next.js inferred your workspace root, but it may not be correct."
  // 'turbopack' is not yet in the typed NextConfig but is supported at runtime
  turbopack: {
    root: __dirname,
  },
  async rewrites() {
    return [
      {
        source: "/ingest/static/:path*",
        destination: "https://us-assets.i.posthog.com/static/:path*",
      },
      {
        source: "/ingest/array/:path*",
        destination: "https://us-assets.i.posthog.com/array/:path*",
      },
      {
        source: "/ingest/:path*",
        destination: "https://us.i.posthog.com/:path*",
      },
    ];
  },
  skipTrailingSlashRedirect: true,
};

export default nextConfig;
