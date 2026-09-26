import type { NextConfig } from "next";
import withPWA from "@ducanh2912/next-pwa";

const baseConfig: NextConfig = {
  transpilePackages: ["@sorolens/ui", "@sorolens/xdr"],
};

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
        // Stale-while-revalidate for the upstream Go API (NEXT_PUBLIC_API_URL)
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
})(baseConfig);
