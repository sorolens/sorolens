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
export function pushPage(
  history: CursorHistory,
  nextCursor: string
): CursorHistory {
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
  to: string
): { since?: string; until?: string } {
  const valid = (d: string) => /^\d{4}-\d{2}-\d{2}$/.test(d);
  const out: { since?: string; until?: string } = {};
  if (valid(from)) out.since = `${from}T00:00:00Z`;
  if (valid(to)) out.until = `${to}T23:59:59Z`;
  return out;
}

/** Validation message for a date range, or null when it is usable. */
export function dateRangeError(from: string, to: string): string | null {
  if (from && to && from > to)
    return "The start date must be on or before the end date.";
  return null;
}

/** Pretty-printed JSON for the expandable data cell. */
export function formatEventData(value: unknown): string {
  if (value === null || value === undefined) return "null";
  try {
    return JSON.stringify(
      value,
      (_key, v) => (typeof v === "bigint" ? v.toString() : v),
      2
    );
  } catch {
    return String(value);
  }
}

/** One-line preview of the event data for the collapsed row. */
export function previewEventData(value: unknown, max = 60): string {
  const compact =
    value === null || value === undefined
      ? "null"
      : formatEventData(value).replace(/\s+/g, " ");
  return compact.length > max ? `${compact.slice(0, max - 1)}…` : compact;
}

// ---- URL-synced state (issue #618) -----------------------------------------
//
// The applied filters and the current page live in the query string
// (?contract=&type=&from=&to=&cursor=&page=) so a filtered view is
// shareable and survives reload.

/** Event type values the explorer accepts; "" means "all types". */
export const EVENT_TYPE_VALUES = ["", "contract", "system", "diagnostic"] as const;

export type EventTypeFilter = (typeof EVENT_TYPE_VALUES)[number];

/** The shareable state of the /events explorer. */
export interface EventsUrlState {
  contractId: string;
  type: EventTypeFilter;
  from: string;
  to: string;
  /** Cursor that loads the current page; "" is the first page. */
  cursor: string;
  /** 1-based page number, for display and for Prev/Next. */
  page: number;
}

export const DEFAULT_EVENTS_URL_STATE: EventsUrlState = {
  contractId: "",
  type: "",
  from: "",
  to: "",
  cursor: "",
  page: 1,
};

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

/**
 * Parses the shareable explorer state from query params. Unknown or
 * malformed values fall back to defaults rather than failing: a mistyped
 * shared link must still show the page.
 */
export function parseEventsUrlState(
  params: URLSearchParams
): EventsUrlState {
  const rawType = params.get("type") ?? "";
  const rawFrom = params.get("from") ?? "";
  const rawTo = params.get("to") ?? "";
  const rawPage = Number.parseInt(params.get("page") ?? "1", 10);
  return {
    contractId: (params.get("contract") ?? "").trim(),
    type: (EVENT_TYPE_VALUES as readonly string[]).includes(rawType)
      ? (rawType as EventTypeFilter)
      : "",
    from: DATE_RE.test(rawFrom) ? rawFrom : "",
    to: DATE_RE.test(rawTo) ? rawTo : "",
    cursor: params.get("cursor") ?? "",
    page: Number.isFinite(rawPage) && rawPage >= 1 ? rawPage : 1,
  };
}

/**
 * Serializes the explorer state back to query params. Default values are
 * omitted so an unfiltered first page has a clean URL.
 */
export function serializeEventsUrlState(
  state: EventsUrlState
): Record<string, string | undefined> {
  return {
    contract: state.contractId || undefined,
    type: state.type || undefined,
    from: state.from || undefined,
    to: state.to || undefined,
    cursor: state.cursor || undefined,
    page: state.page > 1 ? String(state.page) : undefined,
  };
}
