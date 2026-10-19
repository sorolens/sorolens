import { getContractEvents, getContractInvocations } from "@/lib/api";
import type { TimeWindow, VolumePoint } from "@/lib/types";

/**
 * Client-side derivation of the contract detail activity charts (issue #586).
 *
 * The documented stats response carries totals, not time series, so the
 * event-volume and invocation charts are bucketed from the existing paged
 * /events and /invocations endpoints. Both lists are newest-first, so paging
 * stops as soon as the oldest row of a page falls outside the window.
 */

const HOUR_MS = 60 * 60 * 1000;
const DAY_MS = 24 * HOUR_MS;
const PAGE_LIMIT = 200;
const MAX_PAGES = 10;

export type ActivityBucket = "hour" | "day";

export interface ActivityWindowSpec {
  /** Inclusive lower bound; null for "all" (bounded only by MAX_PAGES). */
  since: Date | null;
  bucket: ActivityBucket;
  /** Number of buckets to zero-fill; null for "all". */
  buckets: number | null;
}

export interface ActivitySeries {
  events: VolumePoint[];
  invocations: VolumePoint[];
}

interface TimedRow {
  ledger: number;
  ledger_closed_at: string;
}

/**
 * Resolves a window into bucket size and bounds. Bounds are aligned to bucket
 * starts so zero-filled buckets line up exactly with the data they hold.
 */
export function activityWindow(
  window: TimeWindow,
  now: Date = new Date()
): ActivityWindowSpec {
  if (window === "all") {
    return { since: null, bucket: "day", buckets: null };
  }

  const bucket: ActivityBucket = window === "24h" ? "hour" : "day";
  const buckets = window === "24h" ? 24 : window === "7d" ? 7 : 30;
  const step = bucket === "hour" ? HOUR_MS : DAY_MS;
  const end = Math.floor(now.getTime() / step) * step;
  return {
    since: new Date(end - (buckets - 1) * step),
    bucket,
    buckets,
  };
}

function bucketKey(iso: string, bucket: ActivityBucket): string | null {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return null;
  const normalized = at.toISOString();
  return bucket === "hour"
    ? `${normalized.slice(0, 13)}:00`
    : normalized.slice(0, 10);
}

function bucketLabel(key: string, bucket: ActivityBucket): string {
  return bucket === "hour" ? key.replace("T", " ") : key;
}

/**
 * Aggregates rows into per-hour (24h) or per-day buckets. Bounded windows are
 * zero-filled so the chart always spans the full window; "all" returns only
 * the buckets that contain data, oldest first.
 */
export function aggregateActivity(
  rows: TimedRow[],
  window: TimeWindow,
  now: Date = new Date()
): VolumePoint[] {
  const spec = activityWindow(window, now);
  const counts = new Map<string, { count: number; ledger: number }>();

  for (const row of rows) {
    const at = new Date(row.ledger_closed_at);
    if (Number.isNaN(at.getTime())) continue;
    if (spec.since && at < spec.since) continue;
    const key = bucketKey(row.ledger_closed_at, spec.bucket);
    if (!key) continue;
    const bucket = counts.get(key) ?? { count: 0, ledger: 0 };
    bucket.count += 1;
    bucket.ledger = Math.max(bucket.ledger, row.ledger);
    counts.set(key, bucket);
  }

  let keys: string[];
  if (spec.buckets === null) {
    keys = [...counts.keys()].sort();
  } else {
    const step = spec.bucket === "hour" ? HOUR_MS : DAY_MS;
    const end = Math.floor(now.getTime() / step) * step;
    keys = [];
    for (let i = spec.buckets - 1; i >= 0; i--) {
      const key = bucketKey(
        new Date(end - i * step).toISOString(),
        spec.bucket
      );
      if (key) keys.push(key);
    }
  }

  return keys.map((key) => ({
    date: bucketLabel(key, spec.bucket),
    ledger: counts.get(key)?.ledger ?? 0,
    count: counts.get(key)?.count ?? 0,
  }));
}

async function fetchPages<T extends TimedRow>(
  fetchPage: (
    cursor?: string
  ) => Promise<{ items: T[]; nextCursor: string | null }>,
  since: Date | null
): Promise<T[]> {
  const rows: T[] = [];
  let cursor: string | undefined;

  for (let page = 0; page < MAX_PAGES; page++) {
    const { items, nextCursor } = await fetchPage(cursor);
    rows.push(...items);
    const oldest = items[items.length - 1];
    if (since && oldest && new Date(oldest.ledger_closed_at) < since) break;
    if (!nextCursor) break;
    cursor = nextCursor;
  }

  return rows;
}

/**
 * Fetches the window's events and invocations (paging while rows remain inside
 * the window) and returns both charts' series.
 */
export async function getActivitySeries(
  id: string,
  window: TimeWindow,
  now: Date = new Date()
): Promise<ActivitySeries> {
  const spec = activityWindow(window, now);

  const [events, invocations] = await Promise.all([
    fetchPages(async (cursor) => {
      const res = await getContractEvents(id, {
        limit: PAGE_LIMIT,
        cursor,
      });
      return { items: res.events ?? [], nextCursor: res.next_cursor || null };
    }, spec.since),
    fetchPages(async (cursor) => {
      const res = await getContractInvocations(id, {
        limit: PAGE_LIMIT,
        cursor,
      });
      return {
        items: res.invocations ?? [],
        nextCursor: res.next_cursor || null,
      };
    }, spec.since),
  ]);

  return {
    events: aggregateActivity(events, window, now),
    invocations: aggregateActivity(invocations, window, now),
  };
}
