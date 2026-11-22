import type { NextConfig } from "next";
import path from "node:path";
import withPWA from "@ducanh2912/next-pwa";
import withBundleAnalyzer from "@next/bundle-analyzer";

const baseConfig: NextConfig = {
  outputFileTracingRoot: path.resolve(process.cwd(), "../.."),
  transpilePackages: ["@sorolens/ui", "@sorolens/xdr"],
};

// Bundle analysis (issue #614): run `pnpm build` with ANALYZE=true to emit
// client/server/edge bundle reports (HTML + stats.json) in apps/web/.next.
// The client report is what the performance budget in
// performance-budget.json is measured against; see docs/performance.md.
const analyzeEnabled = process.env.ANALYZE === "true";

export default withPWA({
  dest: "public",
  // Disable in development unless explicitly opted in; always on in production
  disable:
    process.env.NEXT_PUBLIC_PWA_ENABLED !== "true" &&
    process.env.NODE_ENV !== "production",
  // Automatically inject and activate the service worker
  register: true,
  reloadOnOnline: true,
  workboxOptions: {
    runtimeCaching: [
      {
        // Stale-while-revalidate for Next.js API routes (/api/*)
        urlPattern: /^\/api\//,
        handler: "StaleWhileRevalidate",
        options: {
          cacheName: "sorolens-api-cache",
          expiration: {
            maxEntries: 200,
            maxAgeSeconds: 5 * 60, // 5 minutes
          },
          cacheableResponse: { statuses: [0, 200] },
        },
      },
      {
        // Stale-while-revalidate for the upstream Go API (NEXT_PUBLIC_API_URL).
        //
        // EXCEPTION (issue #652): this predicate is serialized by Workbox into
        // the generated service worker, so it cannot close over an import from
        // lib/config.ts — the reference would be undefined in worker scope.
        // The env read + default therefore stays inline here; it must match
        // API_URL in lib/config.ts, which remains the single source of truth
        // for the rest of the app.
        urlPattern: ({ url }) => {
          const apiBase =
            process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
          return url.href.startsWith(apiBase);
        },
        handler: "StaleWhileRevalidate",
        options: {
          cacheName: "sorolens-upstream-api-cache",
          expiration: {
            maxEntries: 400,
            maxAgeSeconds: 5 * 60,
          },
          cacheableResponse: { statuses: [0, 200] },
        },
      },
    ],
  },
})(withBundleAnalyzer({ enabled: analyzeEnabled })(baseConfig));
