import { getContractInvocations } from "@/lib/api";
import type { Invocation, InvocationFrequencyPoint } from "@/lib/types";

/**
 * Client-side aggregation of invocations into hourly counts for the
 * invocation-frequency histogram (issue #185).
 *
 * Buckets are hour-start UTC and zero-filled across the window so the
 * histogram shows a contiguous 24-bar series, matching the semantics of the
 * API's internal RecentHourlyActivity aggregation.
 */

const HOUR_MS = 60 * 60 * 1000;
const PAGE_LIMIT = 200;
const MAX_PAGES = 10;

/** Formats an hour bucket key ("HH:00") from a UTC timestamp. */
function hourKey(iso: string): string {
  return `${iso.slice(11, 13)}:00`;
}

/**
 * Groups invocations into hourly counts over the last `hours` hours,
 * oldest first. Hours with no invocations yield a zero bucket.
 */
export function aggregateInvocationFrequency(
  invocations: Invocation[],
  hours = 24,
  now: Date = new Date()
): InvocationFrequencyPoint[] {
  const counts = new Map<string, number>();
  for (const inv of invocations) {
    const key = hourKey(inv.ledger_closed_at);
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }

  const currentHourMs = Math.floor(now.getTime() / HOUR_MS) * HOUR_MS;
  const points: InvocationFrequencyPoint[] = [];
  for (let i = hours - 1; i >= 0; i--) {
    const hour = hourKey(new Date(currentHourMs - i * HOUR_MS).toISOString());
    points.push({ hour, count: counts.get(hour) ?? 0 });
  }
  return points;
}

/**
 * Fetches the last `hours` hours of invocations (paging through the
 * endpoint) and aggregates them into hourly counts.
 */
export async function getInvocationFrequency(
  id: string,
  hours = 24
): Promise<InvocationFrequencyPoint[]> {
  const since = new Date(Date.now() - hours * HOUR_MS).toISOString();
  const invocations: Invocation[] = [];
  let cursor: string | undefined;

  for (let page = 0; page < MAX_PAGES; page++) {
    const res = await getContractInvocations(id, {
      since,
      limit: PAGE_LIMIT,
      cursor,
    });
    invocations.push(...(res.invocations ?? []));
    if (!res.next_cursor) break;
    cursor = res.next_cursor ?? undefined;
  }

  return aggregateInvocationFrequency(invocations, hours);
}
