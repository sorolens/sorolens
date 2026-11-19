/**
 * URL-synced state for the cross-contract invocations explorer
 * (/invocations, issue #618).
 *
 * The applied filters and the current page live in the query string
 * (?contract=&fn=&since=&until=&cursor=&page=) so a filtered view is
 * shareable and survives reload.
 */

export interface InvocationsUrlState {
  contractId: string;
  fn: string;
  /** Date-picker values (YYYY-MM-DD, UTC days). */
  since: string;
  until: string;
  /** Cursor that loads the current page; "" is the first page. */
  cursor: string;
  /** 1-based page number, for display and for Prev/Next. */
  page: number;
}

export const DEFAULT_INVOCATIONS_URL_STATE: InvocationsUrlState = {
  contractId: "",
  fn: "",
  since: "",
  until: "",
  cursor: "",
  page: 1,
};

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

/**
 * Parses the shareable explorer state from query params. Unknown or
 * malformed values fall back to defaults rather than failing: a mistyped
 * shared link must still show the page.
 */
export function parseInvocationsUrlState(
  params: URLSearchParams
): InvocationsUrlState {
  const rawSince = params.get("since") ?? "";
  const rawUntil = params.get("until") ?? "";
  const rawPage = Number.parseInt(params.get("page") ?? "1", 10);
  return {
    contractId: (params.get("contract") ?? "").trim(),
    fn: (params.get("fn") ?? "").trim(),
    since: DATE_RE.test(rawSince) ? rawSince : "",
    until: DATE_RE.test(rawUntil) ? rawUntil : "",
    cursor: params.get("cursor") ?? "",
    page: Number.isFinite(rawPage) && rawPage >= 1 ? rawPage : 1,
  };
}

/**
 * Serializes the explorer state back to query params. Default values are
 * omitted so an unfiltered first page has a clean URL.
 */
export function serializeInvocationsUrlState(
  state: InvocationsUrlState
): Record<string, string | undefined> {
  return {
    contract: state.contractId || undefined,
    fn: state.fn || undefined,
    since: state.since || undefined,
    until: state.until || undefined,
    cursor: state.cursor || undefined,
    page: state.page > 1 ? String(state.page) : undefined,
  };
}
