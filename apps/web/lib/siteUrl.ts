/**
 * Single source of truth for every absolute site URL (issue #761).
 *
 * `NEXT_PUBLIC_APP_URL` is the only variable read, and `https://sorolens.dev`
 * the only fallback, so `metadataBase`, `robots.txt` and `sitemap.xml` can no
 * longer disagree about the canonical origin of the same deployment. The
 * fallback is a production origin, never localhost: an unset variable should
 * still publish valid URLs.
 */

const SITE_URL_FALLBACK = "https://sorolens.dev";

/** The configured origin with any trailing slash removed, e.g. `https://sorolens.dev`. */
export function siteOrigin(): string {
  const configured = process.env.NEXT_PUBLIC_APP_URL?.trim();
  const origin =
    configured && configured.length > 0 ? configured : SITE_URL_FALLBACK;
  return origin.replace(/\/+$/, "");
}

/**
 * An absolute URL for a site path (`"/contracts"` -> `"<origin>/contracts"`).
 * The site root keeps its trailing slash, which is how a sitemap `<loc>` names
 * a host, and no path ever grows a double slash.
 */
export function absoluteUrl(path = "/"): string {
  const withLead = path.startsWith("/") ? path : `/${path}`;
  const trimmed = withLead.replace(/\/{2,}/g, "/");
  return trimmed === "/" ? `${siteOrigin()}/` : `${siteOrigin()}${trimmed}`;
}
