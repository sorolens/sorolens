/**
 * Shared runtime configuration for the dashboard.
 *
 * `API_URL` is the single source of truth for the SoroLens API base URL. It
 * reads `NEXT_PUBLIC_API_URL` (inlined at build time, so it is safe in client
 * components) and falls back to the Go API's local development default.
 */

export const API_URL: string =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
