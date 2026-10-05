import type { NextConfig } from "next";
import { dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));

const nextConfig: NextConfig = {
  output: 'standalone',
  // Build-time identity of this image, stamped by the release build (see
  // frontend/Dockerfile). Deliberately not NEXT_PUBLIC_* / next-runtime-env: the
  // version is a property of the artifact, so it is inlined and cannot be
  // overridden per environment. Read only via lib/build-info.ts.
  env: {
    APP_BUILD_VERSION: process.env.APP_BUILD_VERSION ?? 'dev',
    APP_BUILD_COMMIT: process.env.APP_BUILD_COMMIT ?? '',
  },
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
