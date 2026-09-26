import type {
  AlertsResponse,
  AlertSubscription,
  ContractDetail,
  CompareResponse,
  ContractSnapshot,
  ContractSummary,
  ContractsListResponse,
  EventsResponse,
  GlobalEventsResponse,
  GlobalStats,
  HealthChecksResponse,
  InvocationsResponse,
  LiveActivityResponse,
  MonitoredContract,
  RecentEventsResponse,
  MonitoredContractsResponse,
  ReportFormat,
  StatsResponse,
  StorageResponse,
  TimeWindow,
  TrackContractRequest,
  ValidateContractResponse,
  SLAHistoryResponse,
  UptimeResponse,
  UptimeWindow,
  WatchdogStats,
  CreateSubscriptionRequest,
  SubscriptionsResponse,
  WatchlistItem,
  WatchlistResponse,
  WatchlistStatusResponse,
  HealthScoreResponse,
  LabelResolution,
} from "./types";
import { recordLastUpdated, resourceFromUrl } from "./lastUpdated";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public code?: string
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function fetchJson<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  if (!res.ok) {
    let body: { error?: string; code?: string } = {};
    try {
      body = await res.json();
    } catch {
      // ignore parse error
    }
    throw new ApiError(res.status, body.error || res.statusText, body.code);
  }

  // A usable payload is on the wire, so the footer can report a fresh view.
  recordLastUpdated(resourceFromUrl(url));

  return res.json();
}

export function listContractsAll(): Promise<ContractsListResponse> {
  return fetchJson<ContractsListResponse>(
    `${API_URL}/api/v1/contracts?limit=1000`
  );
}

export function listContracts(params?: {
  cursor?: string;
  limit?: number;
  network?: string;
  status?: string;
}): Promise<ContractsListResponse> {
  const search = new URLSearchParams();
  if (params?.cursor) search.set("cursor", params.cursor);
  if (params?.limit) search.set("limit", String(params.limit));
  if (params?.network) search.set("network", params.network);
  if (params?.status) search.set("status", params.status);
  const qs = search.toString();
  return fetchJson<ContractsListResponse>(
    `${API_URL}/api/v1/contracts${qs ? "?" + qs : ""}`
  );
}

export function trackContract(
  req: TrackContractRequest,
  userId?: string
): Promise<ContractSummary> {
  const headers: Record<string, string> = {};
  // RBAC: registering a contract requires a recognized (contributor+) user,
  // so the UI forwards its browser identity the same way the watchlist does.
  if (userId) headers["X-User-ID"] = userId;
  return fetchJson<ContractSummary>(`${API_URL}/api/v1/contracts`, {
    method: "POST",
    body: JSON.stringify(req),
    headers,
  });
}

export function getContract(id: string): Promise<ContractDetail> {
  return fetchJson<ContractDetail>(`${API_URL}/api/v1/contracts/${id}`);
}

/**
 * Pre-flight check for the tracking wizard: validates the contract id's StrKey
 * format (including its checksum) and reports whether it is already tracked.
 * Read-only; it never registers the contract.
 */
export function validateContract(req: {
  contract_id: string;
  network: string;
}): Promise<ValidateContractResponse> {
  return fetchJson<ValidateContractResponse>(
    `${API_URL}/api/v1/contracts/validate`,
    { method: "POST", body: JSON.stringify(req) }
  );
}

// ---- live dashboard (#139) -------------------------------------------------

/**
 * The newest events across every tracked contract, newest first. The /live
 * ticker polls this and de-duplicates by event id; there is no cursor because
 * the dashboard always wants the newest slice.
 */
export function getRecentEvents(limit = 50): Promise<RecentEventsResponse> {
  return fetchJson<RecentEventsResponse>(
    `${API_URL}/api/v1/events/recent?limit=${limit}`
  );
}

/**
 * Per-contract events-per-minute buckets over the last `minutes` minutes,
 * ordered hottest first. Backs both the sparklines and the hot-contracts
 * leaderboard in a single request.
 */
export function getLiveActivity(minutes = 30): Promise<LiveActivityResponse> {
  return fetchJson<LiveActivityResponse>(
    `${API_URL}/api/v1/stats/activity?minutes=${minutes}`
  );
}

export function resolveLabel(query: string): Promise<LabelResolution> {
  return fetchJson<LabelResolution>(
    `${API_URL}/api/v1/resolve?query=${encodeURIComponent(query)}`
  );
}

export function getContractEvents(
  id: string,
  params?: {
    cursor?: string;
    limit?: number;
    topic?: string;
    tx_hash?: string;
    since?: string;
    until?: string;
    in_successful_call?: boolean;
  }
): Promise<EventsResponse> {
  const search = new URLSearchParams();
  if (params?.cursor) search.set("cursor", params.cursor);
  if (params?.limit) search.set("limit", String(params.limit));
  if (params?.topic) search.set("topic", params.topic);
  if (params?.tx_hash) search.set("tx_hash", params.tx_hash);
  if (params?.since) search.set("since", params.since);
  if (params?.until) search.set("until", params.until);
  if (params?.in_successful_call !== undefined)
    search.set("in_successful_call", String(params.in_successful_call));
  const qs = search.toString();
  return fetchJson<EventsResponse>(
    `${API_URL}/api/v1/contracts/${id}/events${qs ? "?" + qs : ""}`
  );
}

export function getContractInvocations(
  id: string,
  params?: {
    cursor?: string;
    limit?: number;
    status?: string;
    since?: string;
    until?: string;
    function_name?: string;
  }
): Promise<InvocationsResponse> {
  const search = new URLSearchParams();
  if (params?.cursor) search.set("cursor", params.cursor);
  if (params?.limit) search.set("limit", String(params.limit));
  if (params?.status) search.set("status", params.status);
  if (params?.since) search.set("since", params.since);
  if (params?.until) search.set("until", params.until);
  if (params?.function_name) search.set("function_name", params.function_name);
  const qs = search.toString();
  return fetchJson<InvocationsResponse>(
    `${API_URL}/api/v1/contracts/${id}/invocations${qs ? "?" + qs : ""}`
  );
}

// ---- global invocations ----------------------------------------------------

/**
 * Lists invocations across every tracked contract (newest first). Backs the
 * /invocations explorer. All filters are optional.
 */
export function listInvocations(params?: {
  cursor?: string;
  limit?: number;
  contract_id?: string;
  fn?: string;
  status?: string;
  network?: string;
  since?: string;
  until?: string;
}): Promise<InvocationsResponse> {
  const search = new URLSearchParams();
  if (params?.cursor) search.set("cursor", params.cursor);
  if (params?.limit) search.set("limit", String(params.limit));
  if (params?.contract_id) search.set("contract_id", params.contract_id);
  if (params?.fn) search.set("fn", params.fn);
  if (params?.status) search.set("status", params.status);
  if (params?.network) search.set("network", params.network);
  if (params?.since) search.set("since", params.since);
  if (params?.until) search.set("until", params.until);
  const qs = search.toString();
  return fetchJson<InvocationsResponse>(
    `${API_URL}/api/v1/invocations${qs ? "?" + qs : ""}`
  );
}

export function getContractStorage(
  id: string,
  params?: {
    cursor?: string;
    limit?: number;
    durability?: string;
    status?: string;
    expiring_within?: number;
  }
): Promise<StorageResponse> {
  const search = new URLSearchParams();
  if (params?.cursor) search.set("cursor", params.cursor);
  if (params?.limit) search.set("limit", String(params.limit));
  if (params?.durability) search.set("durability", params.durability);
  if (params?.status) search.set("status", params.status);
  if (params?.expiring_within)
    search.set("expiring_within", String(params.expiring_within));
  const qs = search.toString();
  return fetchJson<StorageResponse>(
    `${API_URL}/api/v1/contracts/${id}/storage${qs ? "?" + qs : ""}`
  );
}

export function getContractStats(
  id: string,
  window: TimeWindow = "7d"
): Promise<StatsResponse> {
  return fetchJson<StatsResponse>(
    `${API_URL}/api/v1/contracts/${id}/stats?window=${window}`
  );
}

// ---- global stats ----------------------------------------------------------

export function getGlobalStats(): Promise<GlobalStats> {
  return fetchJson<GlobalStats>(`${API_URL}/api/v1/stats/global`);
}

// ---- comparison -------------------------------------------------------------

/**
 * Fetch unified stats for up to 4 contracts in a single round-trip. The API
 * fans out to the per-contract lookups in parallel and returns one entry per
 * requested contract; a contract with no data yet still gets an entry with
 * zeroed metrics rather than an error.
 */
export function getCompare(
  ids: string[],
  window: TimeWindow = "7d"
): Promise<CompareResponse> {
  const search = new URLSearchParams();
  search.set("ids", ids.join(","));
  search.set("window", window);
  return fetchJson<CompareResponse>(
    `${API_URL}/api/v1/compare?${search.toString()}`
  );
}

// ---- snapshot / replay ------------------------------------------------------

/**
 * Replay a contract's storage state and last known event as they were at a
 * given ledger. Used by the ledger scrubber on the contract detail page.
 */
export function getContractSnapshot(
  id: string,
  ledger: number
): Promise<ContractSnapshot> {
  return fetchJson<ContractSnapshot>(
    `${API_URL}/api/v1/contracts/${id}/snapshot?ledger=${ledger}`
  );
}

export function getContractHealthScore(
  id: string
): Promise<HealthScoreResponse> {
  return fetchJson<HealthScoreResponse>(
    `${API_URL}/api/v1/contracts/${id}/health-score`
  );
}

// ---- watchdog --------------------------------------------------------------

export function getWatchdogStats(network?: string): Promise<WatchdogStats> {
  const search = new URLSearchParams();
  if (network) search.set("network", network);
  const qs = search.toString();
  return fetchJson<WatchdogStats>(
    `${API_URL}/api/v1/watchdog/stats${qs ? "?" + qs : ""}`
  );
}

export function listMonitoredContracts(params?: {
  cursor?: string;
  limit?: number;
  network?: string;
}): Promise<MonitoredContractsResponse> {
  const search = new URLSearchParams();
  if (params?.cursor) search.set("cursor", params.cursor);
  if (params?.limit) search.set("limit", String(params.limit));
  if (params?.network) search.set("network", params.network);
  const qs = search.toString();
  return fetchJson<MonitoredContractsResponse>(
    `${API_URL}/api/v1/watchdog/contracts${qs ? "?" + qs : ""}`
  );
}

export function getMonitoredContract(
  contractId: string
): Promise<MonitoredContract> {
  return fetchJson<MonitoredContract>(
    `${API_URL}/api/v1/watchdog/contracts/${contractId}`
  );
}

export function getContractUptime(
  contractId: string,
  window: UptimeWindow = "24h"
): Promise<UptimeResponse> {
  return fetchJson<UptimeResponse>(
    `${API_URL}/api/v1/watchdog/contracts/${contractId}/uptime?window=${window}`
  );
}

// ---- SLA reporting (issue #266) --------------------------------------------

/**
 * Monthly SLA buckets for the last `months` calendar months, oldest first.
 * Backs the trend chart in one round-trip instead of a request per month.
 */
export function getContractReportHistory(
  contractId: string,
  months = 12
): Promise<SLAHistoryResponse> {
  const search = new URLSearchParams({ months: String(months) });
  return fetchJson<SLAHistoryResponse>(
    `${API_URL}/api/v1/reports/${contractId}/history?${search.toString()}`
  );
}

/**
 * URL for a report export. Returned rather than fetched so the browser can
 * follow it as a download and honour Content-Disposition.
 */
export function contractReportUrl(
  contractId: string,
  month: string,
  format: ReportFormat
): string {
  const search = new URLSearchParams({ month, format });
  return `${API_URL}/api/v1/reports/${contractId}?${search.toString()}`;
}

/** URL of the embeddable SVG SLA badge for a month. */
export function contractSLABadgeUrl(contractId: string, month: string): string {
  const search = new URLSearchParams({ month });
  return `${API_URL}/api/v1/reports/${contractId}/badge.svg?${search.toString()}`;
}

export function listHealthChecks(
  contractId: string,
  limit = 100
): Promise<HealthChecksResponse> {
  return fetchJson<HealthChecksResponse>(
    `${API_URL}/api/v1/watchdog/contracts/${contractId}/health?limit=${limit}`
  );
}

export function listAlerts(
  contractId?: string,
  params?: {
    severity?: string;
    limit?: number;
    network?: string;
    cursor?: string;
  }
): Promise<AlertsResponse> {
  const search = new URLSearchParams();
  if (params?.severity) search.set("severity", params.severity);
  if (params?.limit) search.set("limit", String(params.limit));
  if (params?.network) search.set("network", params.network);
  if (params?.cursor) search.set("cursor", params.cursor);
  const qs = search.toString();
  const path = contractId
    ? `/api/v1/watchdog/contracts/${contractId}/alerts`
    : `/api/v1/watchdog/alerts`;
  return fetchJson<AlertsResponse>(`${API_URL}${path}${qs ? "?" + qs : ""}`);
}

// ---- events explorer ------------------------------------------------------

export interface ListAllEventsParams {
  cursor?: string;
  limit?: number;
  contractId?: string;
  type?: string;
  network?: string;
  /** RFC 3339 inclusive bounds on ledger_closed_at. */
  since?: string;
  until?: string;
}

/** Cross-contract events feed, newest first (GET /api/v1/events). */
export function listAllEvents(
  params: ListAllEventsParams = {},
  init?: RequestInit
): Promise<GlobalEventsResponse> {
  const search = new URLSearchParams();
  if (params.cursor) search.set("cursor", params.cursor);
  if (params.limit) search.set("limit", String(params.limit));
  if (params.contractId) search.set("contract_id", params.contractId);
  if (params.type) search.set("type", params.type);
  if (params.network) search.set("network", params.network);
  if (params.since) search.set("since", params.since);
  if (params.until) search.set("until", params.until);
  const qs = search.toString();
  return fetchJson<GlobalEventsResponse>(
    `${API_URL}/api/v1/events${qs ? "?" + qs : ""}`,
    init
  );
}

// ---- subscriptions --------------------------------------------------------
// Subscriptions hold integration secrets, so every call needs a contributor
// identity; the dashboard forwards its browser user ID like trackContract.

function userHeaders(userId?: string): Record<string, string> {
  return userId ? { "X-User-ID": userId } : {};
}

export function createSubscription(
  req: CreateSubscriptionRequest,
  userId?: string
): Promise<AlertSubscription> {
  return fetchJson<AlertSubscription>(
    `${API_URL}/api/v1/watchdog/subscriptions`,
    {
      method: "POST",
      body: JSON.stringify(req),
      headers: userHeaders(userId),
    }
  );
}

export function listSubscriptions(
  userId?: string
): Promise<SubscriptionsResponse> {
  return fetchJson<SubscriptionsResponse>(
    `${API_URL}/api/v1/watchdog/subscriptions`,
    {
      headers: userHeaders(userId),
    }
  );
}

export async function deleteSubscription(
  id: string,
  userId?: string
): Promise<void> {
  const res = await fetch(
    `${API_URL}/api/v1/watchdog/subscriptions/${encodeURIComponent(id)}`,
    { method: "DELETE", headers: userHeaders(userId) }
  );
  if (!res.ok) {
    let body: { error?: string | { message?: string } } = {};
    try {
      body = await res.json();
    } catch {
      // ignore parse error
    }
    const message =
      typeof body.error === "string" ? body.error : body.error?.message;
    throw new ApiError(res.status, message || res.statusText);
  }
}

// ---- watchlist ------------------------------------------------------------

export function addToWatchlist(
  contractId: string,
  userId: string
): Promise<WatchlistStatusResponse> {
  return fetchJson<WatchlistStatusResponse>(`${API_URL}/api/v1/watchlist`, {
    method: "POST",
    body: JSON.stringify({ contract_id: contractId }),
    headers: { "X-User-ID": userId },
  });
}

export function removeFromWatchlist(
  contractId: string,
  userId: string
): Promise<WatchlistStatusResponse> {
  return fetchJson<WatchlistStatusResponse>(
    `${API_URL}/api/v1/watchlist/${contractId}`,
    {
      method: "DELETE",
      headers: { "X-User-ID": userId },
    }
  );
}

export function listWatchlist(userId: string): Promise<WatchlistResponse> {
  return fetchJson<WatchlistResponse>(`${API_URL}/api/v1/watchlist`, {
    headers: { "X-User-ID": userId },
  });
}

export function watchlistStatus(
  contractId: string,
  userId: string
): Promise<WatchlistStatusResponse> {
  return fetchJson<WatchlistStatusResponse>(
    `${API_URL}/api/v1/watchlist/${contractId}/status`,
    { headers: { "X-User-ID": userId } }
  );
}
