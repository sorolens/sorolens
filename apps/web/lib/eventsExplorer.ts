/**
 * Pure helpers for the global events explorer (/events).
 *
 * The API paginates with forward-only opaque cursors, so "previous page" is
 * implemented client side by remembering the cursor each visited page was
 * loaded with.
 */

/** Cursors used to load each visited page; index 0 is the first page (""). */
export type CursorHistory = string[];

export const FIRST_PAGE: CursorHistory = [""];

/** The cursor that loads the current page. */
export function currentCursor(history: CursorHistory): string {
  return history[history.length - 1] ?? "";
}

/** Advance to the page loaded by `nextCursor`. */
export function pushPage(history: CursorHistory, nextCursor: string): CursorHistory {
  if (!nextCursor) return history;
  return [...history, nextCursor];
}

/** Go back one page; the first page stays put. */
export function popPage(history: CursorHistory): CursorHistory {
  return history.length > 1 ? history.slice(0, -1) : history;
}

/** 1-based page number for display. */
export function pageNumber(history: CursorHistory): number {
  return history.length;
}

/**
 * Converts date-picker values (YYYY-MM-DD, interpreted as UTC days) into the
 * API's inclusive RFC 3339 bounds: `from` starts at 00:00:00Z and `to` ends
 * at 23:59:59Z. Empty or malformed values are omitted.
 */
export function dateRangeToBounds(
  from: string,
  to: string,
): { since?: string; until?: string } {
  const valid = (d: string) => /^\d{4}-\d{2}-\d{2}$/.test(d);
  const out: { since?: string; until?: string } = {};
  if (valid(from)) out.since = `${from}T00:00:00Z`;
  if (valid(to)) out.until = `${to}T23:59:59Z`;
  return out;
}

/** Validation message for a date range, or null when it is usable. */
export function dateRangeError(from: string, to: string): string | null {
  if (from && to && from > to) return "The start date must be on or before the end date.";
  return null;
}

/** Pretty-printed JSON for the expandable data cell. */
export function formatEventData(value: unknown): string {
  if (value === null || value === undefined) return "null";
  try {
    return JSON.stringify(
      value,
      (_key, v) => (typeof v === "bigint" ? v.toString() : v),
      2,
    );
  } catch {
    return String(value);
  }
}

/** One-line preview of the event data for the collapsed row. */
export function previewEventData(value: unknown, max = 60): string {
  const compact = value === null || value === undefined ? "null" : formatEventData(value).replace(/\s+/g, " ");
  return compact.length > max ? `${compact.slice(0, max - 1)}…` : compact;
}
