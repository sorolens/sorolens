import type {
  ContractDetail,
  ContractsListResponse,
  EventsResponse,
  InvocationsResponse,
  StatsResponse,
  StorageResponse,
  TimeWindow,
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

export function listContracts(
  params?: { network?: string; status?: string },
): Promise<ContractsListResponse> {
  const search = new URLSearchParams();
  if (params?.network) search.set("network", params.network);
  if (params?.status) search.set("status", params.status);
  const qs = search.toString();
  return fetchJson<ContractsListResponse>(
    `${API_URL}/api/v1/contracts${qs ? "?" + qs : ""}`,
  );
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
  },
): Promise<EventsResponse> {
  const search = new URLSearchParams();
  if (params?.cursor) search.set("cursor", params.cursor);
  if (params?.limit) search.set("limit", String(params.limit));
  if (params?.topic) search.set("topic", params.topic);
  if (params?.tx_hash) search.set("tx_hash", params.tx_hash);
  if (params?.since) search.set("since", params.since);
  if (params?.until) search.set("until", params.until);
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
