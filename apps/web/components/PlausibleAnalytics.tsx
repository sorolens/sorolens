/**
 * Privacy-friendly Plausible analytics (issue #191).
 *
 * Emits a single cookieless script tag, and only when both conditions hold:
 *   - the app is running a production build (`NODE_ENV === "production"`), so
 *     local development and the test suite never phone home; and
 *   - `NEXT_PUBLIC_PLAUSIBLE_DOMAIN` is set, naming the site to report to.
 *
 * The script is served from Plausible's CDN rather than bundled, so it adds no
 * measurable weight to the app's JavaScript bundle. No cookies are set.
 */
export function PlausibleAnalytics() {
  const domain = process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN;
  if (process.env.NODE_ENV !== "production" || !domain) {
    return null;
  }
  return (
    <script defer data-domain={domain} src="https://plausible.io/js/script.js" />
  );
}
