import type { HttpMethod, OpenAPIParameter } from "./openapi";

export interface ResponseState {
  status: number;
  statusText: string;
  headers: [string, string][];
  body: string;
  durationMs: number;
}

export function escapeHtml(value: string): string {
  return value.replace(
    /[&<>]/g,
    (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" })[c] as string
  );
}

/**
 * Minimal JSON syntax highlighter. Input is HTML-escaped first, so the only
 * markup in the output is the spans this function adds.
 */
export function highlightJson(text: string): string {
  return escapeHtml(text).replace(
    /("(?:\\.|[^"\\])*")(\s*:)?|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)|\b(true|false|null)\b/g,
    (match, str, colon, num, bool) => {
      if (str !== undefined) {
        return colon !== undefined
          ? `<span class="text-[var(--color-accent)]">${str}</span>${colon}`
          : `<span class="text-emerald-400">${str}</span>`;
      }
      if (num !== undefined) {
        return `<span class="text-amber-400">${num}</span>`;
      }
      if (bool !== undefined) {
        return `<span class="text-fuchsia-400">${bool}</span>`;
      }
      return match;
    }
  );
}

/** Replace {param} segments and append non-empty query params. */
export function buildUrl(
  base: string,
  path: string,
  pathParams: Record<string, string>,
  queryParams: Record<string, string>,
  parameters: OpenAPIParameter[]
): string {
  let resolved = path;
  for (const p of parameters) {
    if (p.in !== "path") continue;
    const value = pathParams[p.name] ?? "";
    resolved = resolved.replace(`{${p.name}}`, encodeURIComponent(value));
  }
  const search = new URLSearchParams();
  for (const p of parameters) {
    if (p.in !== "query") continue;
    const value = queryParams[p.name];
    if (value) search.set(p.name, value);
  }
  const qs = search.toString();
  return `${base}${resolved}${qs ? "?" + qs : ""}`;
}

/** Build a copy-pasteable curl command; the API key is included only if typed. */
export function buildCurl(
  method: HttpMethod,
  url: string,
  apiKey: string,
  body: string
): string {
  const parts = [
    `curl -X ${method} '${url}'`,
    `-H 'Content-Type: application/json'`,
  ];
  if (apiKey) parts.push(`-H 'Authorization: Bearer ${apiKey}'`);
  if (method !== "GET" && body.trim()) parts.push(`-d '${body.trim()}'`);
  return parts.join(" \\\n  ");
}
