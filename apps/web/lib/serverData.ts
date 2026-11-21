import type {
  ContractDetail,
  ContractsListResponse,
  ContractSummary,
  GlobalStats,
  StatsResponse,
  WatchdogStats,
} from "@/lib/types";
import { API_URL as DEFAULT_API_URL } from "@/lib/config";

// Server-side base: NEXT_PUBLIC_API_URL still wins over the server-only
// API_URL override, then the shared default; a trailing slash is stripped so
// `${API_URL}${path}` never produces a double slash.
const API_URL = (
  process.env.NEXT_PUBLIC_API_URL ??
  process.env.API_URL ??
  DEFAULT_API_URL
).replace(/\/$/, "");

async function fetchServerData<T>(path: string): Promise<T | null> {
  try {
    const response = await fetch(`${API_URL}${path}`, {
      headers: { Accept: "application/json" },
      next: { revalidate: 60 },
    });
    if (!response.ok) {
      if (response.status >= 500) {
        console.error(
          `Server data request failed with status ${response.status}: ${path}`
        );
      }
      return null;
    }
    return (await response.json()) as T;
  } catch (error) {
    console.error(`Server data request failed: ${path}`, error);
    return null;
  }
}

export function getLandingStats() {
  return Promise.all([
    fetchServerData<GlobalStats>("/api/v1/stats/global"),
    fetchServerData<WatchdogStats>("/api/v1/watchdog/stats"),
  ]).then(([global, watchdog]) => ({ global, watchdog }));
}

export function getInitialContracts(
  sort = "added_at",
  dir: "asc" | "desc" = "desc"
): Promise<ContractsListResponse | null> {
  return fetchServerData<ContractsListResponse>(
    `/api/v1/contracts?limit=20&sort=${encodeURIComponent(sort)}&dir=${dir}`
  );
}

/**
 * Every tracked contract, for `sitemap.xml` (issue #761).
 *
 * Pages through `next_cursor` rather than asking for one large page: the API
 * has no upper bound on `limit`, so an unbounded request would be a way to make
 * the sitemap cost the database an arbitrary amount of work. The page count is
 * capped instead, so the sitemap covers the first
 * `SITEMAP_MAX_PAGES * SITEMAP_PAGE_SIZE` contracts and a deployment larger than
 * that is served by a sitemap index.
 */
const SITEMAP_PAGE_SIZE = 100;
const SITEMAP_MAX_PAGES = 20;

export async function getSitemapContracts(): Promise<ContractSummary[]> {
  const contracts: ContractSummary[] = [];
  let cursor: string | undefined;

  for (let page = 0; page < SITEMAP_MAX_PAGES; page++) {
    const query = new URLSearchParams({
      limit: String(SITEMAP_PAGE_SIZE),
      sort: "added_at",
      dir: "desc",
    });
    if (cursor !== undefined) query.set("cursor", cursor);

    const res = await fetchServerData<ContractsListResponse>(
      `/api/v1/contracts?${query.toString()}`
    );
    if (!res) break;
    contracts.push(...res.contracts);

    cursor = res.next_cursor || undefined;
    if (cursor === undefined) break;
  }

  return contracts;
}

export function getContractForMetadata(
  id: string
): Promise<ContractDetail | null> {
  return fetchServerData<ContractDetail>(
    `/api/v1/contracts/${encodeURIComponent(id)}`
  );
}

export function getContractStatsForMetadata(
  id: string
): Promise<StatsResponse | null> {
  return fetchServerData<StatsResponse>(
    `/api/v1/contracts/${encodeURIComponent(id)}/stats?window=7d`
  );
}
