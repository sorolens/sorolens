import type { MetadataRoute } from "next";

const SITE_URL_FALLBACK = "https://sorolens.dev";

/**
 * Canonical origin advertised in `robots.txt`. Derived from the environment so
 * dev, staging, and production each publish their own sitemap/host instead of a
 * hardcoded domain, with the production origin as a fallback.
 */
function siteUrl(): string {
  return (process.env.NEXT_PUBLIC_APP_URL ?? SITE_URL_FALLBACK).replace(/\/$/, "");
}

export default function robots(): MetadataRoute.Robots {
  const origin = siteUrl();

  return {
    rules: {
      userAgent: "*",
      allow: "/",
    },
    sitemap: `${origin}/sitemap.xml`,
    host: origin,
  };
}
