import {
  ApiError,
  getContract,
  getContractHealthScore,
  getContractStats,
} from "@/lib/api";
import { getActivitySeries } from "@/lib/activitySeries";
import type { CompareContractEntry, CompareVolumePoint } from "@/lib/types";

/**
 * Comparison entries for the "all" time window (issue #607).
 *
 * GET /api/v1/compare only aggregates the documented 24h/7d/30d windows —
 * unknown values are silently normalised to 24h — and its event/invocation
 * counts are window-scoped. Lifetime numbers, however, are already reachable
 * from the client: the stats endpoint carries overall totals (that is what the
 * detail page shows when its selector is on "All"), and the activity-series
 * helper pages the full event history for the "all" charts. This module
 * assembles the same CompareContractEntry shape CompareColumn consumes from
 * those calls, so /compare can support "all" honestly instead of relabelling
 * 24h data.
 *
 * Lifetime avg CPU/fee are not exposed by any endpoint, so they stay null and
 * render as "—" rather than as a misleading zero.
 */
export function getCompareAllEntries(
  ids: string[]
): Promise<CompareContractEntry[]> {
  return Promise.all(ids.map(getCompareAllEntry));
}

async function getCompareAllEntry(id: string): Promise<CompareContractEntry> {
  const entry: CompareContractEntry = {
    id,
    network: "",
    label: "",
    status: "unknown",
    tracked: false,
    has_data: false,
    event_count: 0,
    invocation_count: 0,
    avg_cpu: null,
    avg_fee: null,
    health_score: null,
    last_synced_ledger: 0,
    event_volume: [],
  };

  let detail;
  try {
    detail = await getContract(id);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      // Not registered: a present, zeroed entry, mirroring the API's
      // compareOne, so one unknown contract never fails the comparison.
      return entry;
    }
    entry.error = "failed to load contract";
    return entry;
  }
  entry.network = detail.network;
  entry.label = detail.label ?? "";
  entry.status = detail.status;
  entry.tracked = true;

  try {
    // The window parameter only drives the window_* counts; the totals we
    // need are in every response. Same "all" mapping the detail page uses.
    const stats = await getContractStats(id, "24h");
    entry.event_count = stats.event_count;
    entry.invocation_count = stats.invocation_count;
    entry.last_synced_ledger = stats.last_synced_ledger;
  } catch {
    entry.error = "failed to load stats";
    return entry;
  }

  try {
    const series = await getActivitySeries(id, "all");
    entry.event_volume = series.events.map<CompareVolumePoint>((p) => ({
      timestamp: p.date,
      count: p.count,
    }));
  } catch {
    // The sparkline is secondary; the metric rows stay comparable without it.
  }

  try {
    const health = await getContractHealthScore(id);
    entry.health_score = health.score;
  } catch (err) {
    if (!(err instanceof ApiError) || err.status !== 404) {
      entry.error = "failed to load health score";
      return entry;
    }
    // No score computed yet: null, exactly like the API's compareOne.
  }

  entry.has_data =
    entry.tracked &&
    (entry.event_count > 0 ||
      entry.invocation_count > 0 ||
      entry.health_score !== null);
  return entry;
}
