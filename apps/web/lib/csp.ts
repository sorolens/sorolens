/**
 * Content-Security-Policy for the dashboard (issue #836). The policy is built
 * per request by middleware.ts; see "Content-Security-Policy" in the README
 * for how to extend it.
 */

/** Request header carrying the nonce from middleware to the root layout. */
export const NONCE_HEADER = "x-nonce";

/** Same-origin endpoint browsers deliver violation reports to. */
export const CSP_REPORT_PATH = "/api/csp-report";

/** Reporting API group name used by `report-to`. */
export const CSP_REPORT_GROUP = "csp-endpoint";

export interface CspOptions {
  nonce: string;
  /** `next dev` needs eval and a hot-reload socket; production never gets them. */
  isDev: boolean;
  apiUrl: string;
  sentryDsn?: string;
  plausibleDomain?: string;
}

/** Base64 encoding of 128 random bits. */
export function generateNonce(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return btoa(String.fromCharCode(...bytes));
}

function originOf(url: string | undefined): string | null {
  if (!url) return null;
  try {
    return new URL(url).origin;
  } catch {
    return null;
  }
}

export function buildCsp({
  nonce,
  isDev,
  apiUrl,
  sentryDsn,
  plausibleDomain,
}: CspOptions): string {
  const apiOrigin = originOf(apiUrl);
  const sentryOrigin = originOf(sentryDsn);

  const connectSrc = ["'self'"];
  if (apiOrigin) {
    // Live updates use a WebSocket to the same host as the REST API.
    connectSrc.push(apiOrigin, apiOrigin.replace(/^http/, "ws"));
  }
  if (sentryOrigin) connectSrc.push(sentryOrigin);
  if (plausibleDomain) connectSrc.push("https://plausible.io");
  if (isDev) connectSrc.push("ws://localhost:*", "ws://127.0.0.1:*");

  const imgSrc = ["'self'", "data:", "blob:"];
  if (apiOrigin) imgSrc.push(apiOrigin);

  const scriptSrc = ["'self'", `'nonce-${nonce}'`, "'strict-dynamic'"];
  if (isDev) scriptSrc.push("'unsafe-eval'");

  // `next dev` injects its stylesheets as <style> elements without a nonce.
  // A nonce makes browsers ignore 'unsafe-inline', so development drops the
  // nonce for styles instead; production keeps <style> nonced.
  const styleSrc = isDev
    ? ["'self'", "'unsafe-inline'"]
    : ["'self'", `'nonce-${nonce}'`];

  const directives: Record<string, string[]> = {
    "default-src": ["'self'"],
    "script-src": scriptSrc,
    "style-src": styleSrc,
    // Style attributes cannot run code, and React and Recharts server-render
    // them for layout, so only attributes are exempt. <style> stays nonced.
    "style-src-attr": ["'unsafe-inline'"],
    "img-src": imgSrc,
    "font-src": ["'self'", "data:"],
    "connect-src": connectSrc,
    "worker-src": ["'self'", "blob:"],
    "manifest-src": ["'self'"],
    "object-src": ["'none'"],
    "base-uri": ["'self'"],
    "form-action": ["'self'"],
    "frame-ancestors": ["'none'"],
    "report-uri": [CSP_REPORT_PATH],
    "report-to": [CSP_REPORT_GROUP],
  };

  return Object.entries(directives)
    .map(([name, values]) => `${name} ${values.join(" ")}`)
    .join("; ");
}

/** `CSP_REPORT_ONLY=true` reports violations without blocking anything. */
export function cspHeaderName(reportOnly: string | undefined): string {
  return reportOnly === "true"
    ? "Content-Security-Policy-Report-Only"
    : "Content-Security-Policy";
}
