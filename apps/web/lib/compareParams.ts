import type { TimeWindow } from "@/lib/types";

const SUPPORTED_WINDOWS: TimeWindow[] = ["24h", "7d", "30d", "all"];
const DEFAULT_WINDOW: TimeWindow = "7d";
const MAX_IDS = 4;

// Soroban contract id: 56 chars, 'C' + base32 alphabet — the same shape the
// API rejects with 422 (parseCompareIDs) and that the tracking wizard and CSV
// import validate client-side. A malformed id in the URL used to be forwarded
// to GET /compare, failing the whole comparison for one typo (#607).
const CONTRACT_ID_RE = /^C[A-Z0-9]{55}$/;

export interface ParsedIds {
  /** Valid, upper-cased, de-duplicated ids, capped at MAX_IDS. */
  ids: string[];
  /** Ids dropped because they are malformed. */
  dropped: number;
  /** Valid ids dropped because more than MAX_IDS were supplied. */
  truncated: number;
}

/**
 * Splits ?ids=A,B, validates each part, and counts what had to be dropped so
 * the view can tell the user instead of failing silently. Lowercase input is
 * canonicalised to upper case, matching the wizard's paste handling.
 */
export function parseIds(raw: string | undefined): ParsedIds {
  const ids: string[] = [];
  const seen = new Set<string>();
  const result: ParsedIds = { ids, dropped: 0, truncated: 0 };
  if (!raw) return result;
  for (const part of raw.split(",")) {
    const id = part.trim().toUpperCase();
    if (!id) continue;
    if (!CONTRACT_ID_RE.test(id)) {
      result.dropped++;
      continue;
    }
    if (seen.has(id)) continue;
    seen.add(id);
    if (ids.length === MAX_IDS) {
      result.truncated++;
      continue;
    }
    ids.push(id);
  }
  return result;
}

export function parseWindow(raw: string | undefined): TimeWindow {
  return SUPPORTED_WINDOWS.includes(raw as TimeWindow)
    ? (raw as TimeWindow)
    : DEFAULT_WINDOW;
}
