/**
 * URL-synced state for the contracts list (/contracts, issue #618).
 *
 * The free-text search, the tag filter, the sort and the current page live
 * in the query string (?q=&tag=&sort=&dir=&cursor=&page=) so a view is
 * shareable and survives reload. Sort was already URL-driven (#175); this
 * extends the same treatment to the filters and pagination.
 */

/** Column keys the list can sort by; must match the backend whitelist. */
export const CONTRACT_SORT_COLUMNS = [
  "id",
  "network",
  "status",
  "label",
  "added_at",
  "last_activity_at",
] as const;

export type ContractSortColumn = (typeof CONTRACT_SORT_COLUMNS)[number];

export const DEFAULT_CONTRACT_SORT: ContractSortColumn = "added_at";
export const DEFAULT_CONTRACT_SORT_DIRECTION: "asc" | "desc" = "desc";

export interface ContractsUrlState {
  /** Free-text client-side search over the current page. */
  q: string;
  /** Server-side tag filter. */
  tag: string;
  sort: ContractSortColumn;
  dir: "asc" | "desc";
  /** Cursor that loads the current page; "" is the first page. */
  cursor: string;
  /** 1-based page number, for display and for Prev/Next. */
  page: number;
}

export const DEFAULT_CONTRACTS_URL_STATE: ContractsUrlState = {
  q: "",
  tag: "",
  sort: DEFAULT_CONTRACT_SORT,
  dir: DEFAULT_CONTRACT_SORT_DIRECTION,
  cursor: "",
  page: 1,
};

/**
 * Parses the shareable list state from query params. Unknown or malformed
 * values fall back to defaults rather than failing: a mistyped shared link
 * must still show the page.
 */
export function parseContractsUrlState(
  params: URLSearchParams
): ContractsUrlState {
  const rawSort = params.get("sort") ?? "";
  const rawDir = params.get("dir") ?? "";
  const rawPage = Number.parseInt(params.get("page") ?? "1", 10);
  return {
    q: params.get("q") ?? "",
    tag: (params.get("tag") ?? "").trim(),
    sort: (CONTRACT_SORT_COLUMNS as readonly string[]).includes(rawSort)
      ? (rawSort as ContractSortColumn)
      : DEFAULT_CONTRACT_SORT,
    dir: rawDir === "asc" ? "asc" : "desc",
    cursor: params.get("cursor") ?? "",
    page: Number.isFinite(rawPage) && rawPage >= 1 ? rawPage : 1,
  };
}

/**
 * Serializes the list state back to query params. Default values are
 * omitted so an unfiltered, default-sorted first page has a clean URL.
 * Sort and dir are written together whenever either is non-default: a
 * lone ?dir=asc with an implied sort is needlessly confusing to share.
 */
export function serializeContractsUrlState(
  state: ContractsUrlState
): Record<string, string | undefined> {
  const defaultSort =
    state.sort === DEFAULT_CONTRACT_SORT &&
    state.dir === DEFAULT_CONTRACT_SORT_DIRECTION;
  return {
    q: state.q || undefined,
    tag: state.tag || undefined,
    sort: defaultSort ? undefined : state.sort,
    dir: defaultSort ? undefined : state.dir,
    cursor: state.cursor || undefined,
    page: state.page > 1 ? String(state.page) : undefined,
  };
}
