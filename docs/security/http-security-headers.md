# HTTP security headers

The API router applies these headers to every response, including rejected
requests and CORS preflight responses:

- `X-Content-Type-Options: nosniff` prevents browsers from MIME-sniffing a
  response away from its declared content type.
- `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'`
  provides a deny-by-default baseline for any browser-rendered API response,
  prevents the response from being embedded, and prevents a hostile base URL
  from changing relative URL resolution.
- `Strict-Transport-Security: max-age=31536000; includeSubDomains` tells
  user agents to use HTTPS for the API origin and its subdomains for one year.
  The deployment must serve the API exclusively over HTTPS before enabling this
  policy in production.
- `Referrer-Policy: strict-origin-when-cross-origin` limits cross-origin
  referrers to the origin while retaining the full URL for same-origin
  requests.

API-key management and admin endpoints under `/api/v1` and `/api/v2` also send
`Cache-Control: no-store`. This covers both the `/api-keys` routes and the
`/admin` route groups, including error responses, so credentials and key
metadata are not retained by browsers or intermediary caches.

The implementation lives in `apps/api/internal/middleware/security_headers.go`
and is registered as global router middleware in
`apps/api/internal/router/router.go`.
