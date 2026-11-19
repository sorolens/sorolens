import { NextResponse, type NextRequest } from "next/server";
import {
  createRateLimiter,
  MAX_REPORT_BYTES,
  parseCspReports,
} from "@/lib/cspReport";

export const dynamic = "force-dynamic";

const ACCEPTED_TYPES = new Set([
  "application/csp-report",
  "application/reports+json",
  "application/json",
]);

const allowReport = createRateLimiter({
  limit: 30,
  windowMs: 60_000,
  maxClients: 10_000,
});

function reject(status: number, error: string) {
  return NextResponse.json({ error }, { status });
}

/** Receives CSP violation reports and logs a sanitised summary of each. */
export async function POST(request: NextRequest) {
  const client =
    request.headers.get("x-forwarded-for")?.split(",")[0].trim() || "unknown";
  if (!allowReport(client)) {
    return reject(429, "too many reports");
  }

  const contentType = request.headers
    .get("content-type")
    ?.split(";")[0]
    .trim()
    .toLowerCase();
  if (!contentType || !ACCEPTED_TYPES.has(contentType)) {
    return reject(415, "unsupported content type");
  }

  if (Number(request.headers.get("content-length") ?? 0) > MAX_REPORT_BYTES) {
    return reject(413, "report too large");
  }
  const raw = await request.text();
  if (new TextEncoder().encode(raw).length > MAX_REPORT_BYTES) {
    return reject(413, "report too large");
  }

  let payload: unknown;
  try {
    payload = JSON.parse(raw);
  } catch {
    return reject(400, "invalid report");
  }
  const reports = parseCspReports(payload);
  if (!reports) {
    return reject(400, "invalid report");
  }

  for (const report of reports) {
    console.warn("csp-violation", JSON.stringify(report));
  }
  return new NextResponse(null, { status: 204 });
}
