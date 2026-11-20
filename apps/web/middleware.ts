import { NextResponse, type NextRequest } from "next/server";
import {
  buildCsp,
  CSP_REPORT_GROUP,
  CSP_REPORT_PATH,
  cspHeaderName,
  generateNonce,
  NONCE_HEADER,
} from "@/lib/csp";
import { API_URL } from "@/lib/config";

/**
 * Issues a fresh CSP nonce for every document request (issue #836). Next.js
 * reads the policy from the request headers and applies the nonce to its own
 * scripts; the root layout reads NONCE_HEADER for the ones it renders itself.
 */
export function middleware(request: NextRequest) {
  const nonce = generateNonce();
  const policy = buildCsp({
    nonce,
    isDev: process.env.NODE_ENV === "development",
    apiUrl: API_URL,
    sentryDsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
    plausibleDomain: process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN,
  });
  const headerName = cspHeaderName(process.env.CSP_REPORT_ONLY);

  const requestHeaders = new Headers(request.headers);
  requestHeaders.set(NONCE_HEADER, nonce);
  requestHeaders.set(headerName, policy);

  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set(headerName, policy);
  response.headers.set(
    "Reporting-Endpoints",
    `${CSP_REPORT_GROUP}="${CSP_REPORT_PATH}"`
  );
  return response;
}

export const config = {
  matcher: [
    {
      // Documents only: API routes, build output and public files carry no
      // inline scripts, and prefetches reuse the nonce of the page they run on.
      source:
        "/((?!api/|_next/static|_next/image|.*\\.(?:ico|png|svg|jpg|jpeg|webp|json|txt|xml|js|map|webmanifest)$).*)",
      missing: [
        { type: "header", key: "next-router-prefetch" },
        { type: "header", key: "purpose", value: "prefetch" },
      ],
    },
  ],
};
