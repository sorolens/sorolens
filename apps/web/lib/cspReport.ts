/**
 * Validation for browser CSP violation reports (issue #836). Reports are
 * untrusted input and can embed URLs with personal data, so only a small,
 * sanitised summary ever leaves this module.
 */

/** Largest report body accepted, in bytes. */
export const MAX_REPORT_BYTES = 16 * 1024;

const MAX_REPORTS_PER_BATCH = 20;
const MAX_FIELD_LENGTH = 2048;
const DIRECTIVE_PATTERN = /^[a-z][a-z-]{1,40}$/;
const KEYWORD_SOURCES = new Set([
  "inline",
  "eval",
  "wasm-eval",
  "trusted-types-policy",
  "trusted-types-sink",
  "self",
  "data",
  "blob",
]);

export interface CspViolationSummary {
  directive: string;
  blocked: string;
  documentPath: string;
  disposition: "enforce" | "report" | "unknown";
}

function text(value: unknown): string | null {
  return typeof value === "string" && value.length <= MAX_FIELD_LENGTH
    ? value
    : null;
}

/** Reduces a blocked resource to its origin or a CSP keyword such as inline. */
function sanitizeBlocked(value: string | null): string {
  if (!value) return "unknown";
  if (KEYWORD_SOURCES.has(value)) return value;
  try {
    const url = new URL(value);
    if (url.protocol === "http:" || url.protocol === "https:") {
      return url.origin;
    }
    if (url.protocol === "ws:" || url.protocol === "wss:") return url.origin;
    return url.protocol.replace(/:$/, "");
  } catch {
    return "unknown";
  }
}

/** Keeps the path only: query strings and fragments can carry user data. */
function sanitizeDocument(value: string | null): string | null {
  if (!value) return null;
  try {
    return new URL(value).pathname;
  } catch {
    return null;
  }
}

function summarize(fields: {
  directive: unknown;
  blocked: unknown;
  document: unknown;
  disposition: unknown;
}): CspViolationSummary | null {
  const directive = text(fields.directive)?.split(" ")[0];
  const documentPath = sanitizeDocument(text(fields.document));
  if (!directive || !DIRECTIVE_PATTERN.test(directive) || !documentPath) {
    return null;
  }
  return {
    directive,
    blocked: sanitizeBlocked(text(fields.blocked)),
    documentPath,
    disposition:
      fields.disposition === "enforce" || fields.disposition === "report"
        ? fields.disposition
        : "unknown",
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * Accepts both wire formats: the legacy `report-uri` object and the Reporting
 * API batch. Returns null when the payload is not a valid report.
 */
export function parseCspReports(
  payload: unknown
): CspViolationSummary[] | null {
  if (Array.isArray(payload)) {
    if (payload.length === 0 || payload.length > MAX_REPORTS_PER_BATCH) {
      return null;
    }
    const summaries: CspViolationSummary[] = [];
    for (const entry of payload) {
      if (
        !isRecord(entry) ||
        entry.type !== "csp-violation" ||
        !isRecord(entry.body)
      ) {
        return null;
      }
      const summary = summarize({
        directive: entry.body.effectiveDirective,
        blocked: entry.body.blockedURL,
        document: entry.body.documentURL,
        disposition: entry.body.disposition,
      });
      if (!summary) return null;
      summaries.push(summary);
    }
    return summaries;
  }

  if (!isRecord(payload) || !isRecord(payload["csp-report"])) return null;
  const report = payload["csp-report"];
  const summary = summarize({
    directive: report["effective-directive"] ?? report["violated-directive"],
    blocked: report["blocked-uri"],
    document: report["document-uri"],
    disposition: report.disposition,
  });
  return summary ? [summary] : null;
}

export interface RateLimiterOptions {
  limit: number;
  windowMs: number;
  /** Upper bound on tracked clients, so the map cannot grow without limit. */
  maxClients: number;
}

/** Fixed-window, per-instance limiter. Keys stay in memory and are not logged. */
export function createRateLimiter({
  limit,
  windowMs,
  maxClients,
}: RateLimiterOptions) {
  const clients = new Map<string, { count: number; resetAt: number }>();

  return function allow(key: string, now: number = Date.now()): boolean {
    const current = clients.get(key);
    if (!current || current.resetAt <= now) {
      if (!current && clients.size >= maxClients) {
        for (const [client, entry] of clients) {
          if (entry.resetAt <= now) clients.delete(client);
        }
        // Still full of live windows: treat further new clients as limited.
        if (clients.size >= maxClients) return false;
      }
      clients.set(key, { count: 1, resetAt: now + windowMs });
      return true;
    }
    current.count += 1;
    return current.count <= limit;
  };
}
