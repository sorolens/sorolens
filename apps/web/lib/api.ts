import type {
  AlertsResponse,
  AlertSubscription,
  ContractDetail,
  CompareResponse,
  ContractSnapshot,
  ContractSummary,
  ContractsListResponse,
  EventsResponse,
  GlobalStats,
  HealthChecksResponse,
  InvocationsResponse,
  MonitoredContract,
  MonitoredContractsResponse,
  StatsResponse,
  StorageResponse,
  TimeWindow,
  TrackContractRequest,
  WatchdogStats,
  CreateSubscriptionRequest,
  SubscriptionsResponse,
  WatchlistItem,
  WatchlistResponse,
  WatchlistStatusResponse,
  HealthScoreResponse,
} from "./types";


const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public code?: string,
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

  return res.json();
}

export function listContractsAll(): Promise<ContractsListResponse> {
  return fetchJson<ContractsListResponse>(
    `${API_URL}/api/v1/contracts?limit=1000`,
  );
}

export function listContracts(
  params?: { cursor?: string; limit?: number; network?: string; status?: string },
): Promise<ContractsListResponse> {
  const search = new URLSearchParams();
  if (params?.cursor) search.set("cursor", params.cursor);
  if (params?.limit) search.set("limit", String(params.limit));
  if (params?.network) search.set("network", params.network);
  if (params?.status) search.set("status", params.status);
  const qs = search.toString();
  return fetchJson<ContractsListResponse>(
    `${API_URL}/api/v1/contracts${qs ? "?" + qs : ""}`,
  );
}

export function trackContract(
  req: TrackContractRequest,
  userId?: string,
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
  },
): Promise<EventsResponse> {
  const search = new URLSearchParams();
  if (params?.cursor) search.set("cursor", params.cursor);
  if (params?.limit) search.set("limit", String(params.limit));
  if (params?.topic) search.set("topic", params.topic);
  if (params?.tx_hash) search.set("tx_hash", params.tx_hash);
  if (params?.since) search.set("since", params.since);
  if (params?.until) search.set("until", params.until);
  if (params?.in_successful_call !== undefined) search.set("in_successful_call", String(params.in_successful_call));
  const qs = search.toString();
  return fetchJson<EventsResponse>(
    `${API_URL}/api/v1/contracts/${id}/events${qs ? "?" + qs : ""}`,
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
  },
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
    `${API_URL}/api/v1/contracts/${id}/invocations${qs ? "?" + qs : ""}`,
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
  },
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
    `${API_URL}/api/v1/contracts/${id}/storage${qs ? "?" + qs : ""}`,
  );
}

export function getContractStats(
  id: string,
  window: TimeWindow = "7d",
): Promise<StatsResponse> {
  return fetchJson<StatsResponse>(
    `${API_URL}/api/v1/contracts/${id}/stats?window=${window}`,
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
  window: TimeWindow = "7d",
): Promise<CompareResponse> {
  const search = new URLSearchParams();
  search.set("ids", ids.join(","));
  search.set("window", window);
  return fetchJson<CompareResponse>(
    `${API_URL}/api/v1/compare?${search.toString()}`,
  );
}

// ---- snapshot / replay ------------------------------------------------------

/**
 * Replay a contract's storage state and last known event as they were at a
 * given ledger. Used by the ledger scrubber on the contract detail page.
 */
export function getContractSnapshot(
  id: string,
  ledger: number,
): Promise<ContractSnapshot> {
  return fetchJson<ContractSnapshot>(
    `${API_URL}/api/v1/contracts/${id}/snapshot?ledger=${ledger}`,
  );
}


export function getContractHealthScore(
  id: string,
): Promise<HealthScoreResponse> {
  return fetchJson<HealthScoreResponse>(
    `${API_URL}/api/v1/contracts/${id}/health-score`,
  );
}

// ---- watchdog --------------------------------------------------------------

export function getWatchdogStats(network?: string): Promise<WatchdogStats> {
  const search = new URLSearchParams();
  if (network) search.set("network", network);
  const qs = search.toString();
  return fetchJson<WatchdogStats>(
    `${API_URL}/api/v1/watchdog/stats${qs ? "?" + qs : ""}`,
  );
}

export function listMonitoredContracts(
  params?: { cursor?: string; limit?: number; network?: string },
): Promise<MonitoredContractsResponse> {
  const search = new URLSearchParams();
  if (params?.cursor) search.set("cursor", params.cursor);
  if (params?.limit) search.set("limit", String(params.limit));
  if (params?.network) search.set("network", params.network);
  const qs = search.toString();
  return fetchJson<MonitoredContractsResponse>(
    `${API_URL}/api/v1/watchdog/contracts${qs ? "?" + qs : ""}`,
  );
}

export function getMonitoredContract(
  contractId: string,
): Promise<MonitoredContract> {
  return fetchJson<MonitoredContract>(
    `${API_URL}/api/v1/watchdog/contracts/${contractId}`,
  );
}

export function listHealthChecks(
  contractId: string,
  limit = 100,
): Promise<HealthChecksResponse> {
  return fetchJson<HealthChecksResponse>(
    `${API_URL}/api/v1/watchdog/contracts/${contractId}/health?limit=${limit}`,
  );
}

export function listAlerts(
  contractId?: string,
  params?: {
    severity?: string;
    limit?: number;
    network?: string;
    cursor?: string;
  },
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

// ---- subscriptions --------------------------------------------------------

export function createSubscription(
  req: CreateSubscriptionRequest,
): Promise<AlertSubscription> {
  return fetchJson<AlertSubscription>(`${API_URL}/api/v1/watchdog/subscriptions`, {
    method: "POST",
    body: JSON.stringify(req),
  });
}

export function listSubscriptions(): Promise<SubscriptionsResponse> {
  return fetchJson<SubscriptionsResponse>(`${API_URL}/api/v1/watchdog/subscriptions`);
}

export function deleteSubscription(id: string): Promise<void> {
  return fetchJson<void>(`${API_URL}/api/v1/watchdog/subscriptions/${id}`, {
    method: "DELETE",
  });
}

// ---- watchlist ------------------------------------------------------------

export function addToWatchlist(
  contractId: string,
  userId: string,
): Promise<WatchlistStatusResponse> {
  return fetchJson<WatchlistStatusResponse>(`${API_URL}/api/v1/watchlist`, {
    method: "POST",
    body: JSON.stringify({ contract_id: contractId }),
    headers: { "X-User-ID": userId },
  });
}

export function removeFromWatchlist(
  contractId: string,
  userId: string,
): Promise<WatchlistStatusResponse> {
  return fetchJson<WatchlistStatusResponse>(`${API_URL}/api/v1/watchlist/${contractId}`, {
    method: "DELETE",
    headers: { "X-User-ID": userId },
  });
}

export function listWatchlist(userId: string): Promise<WatchlistResponse> {
  return fetchJson<WatchlistResponse>(`${API_URL}/api/v1/watchlist`, {
    headers: { "X-User-ID": userId },
  });
}

export function watchlistStatus(
  contractId: string,
  userId: string,
): Promise<WatchlistStatusResponse> {
  return fetchJson<WatchlistStatusResponse>(
    `${API_URL}/api/v1/watchlist/${contractId}/status`,
    { headers: { "X-User-ID": userId } },
  );
}
