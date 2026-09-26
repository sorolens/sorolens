import { getContractInvocations } from "@/lib/api";
import type { Invocation, ResourceTrendPoint } from "@/lib/types";

/**
 * Client-side aggregation of per-invocation resource usage into daily
 * averages for the resource-usage trend chart (issue #184).
 *
 * The chart is fed from the existing /invocations endpoint rather than a new
 * /resource-trend endpoint, keeping the REST surface unchanged.
 */

const DAY_MS = 24 * 60 * 60 * 1000;
const PAGE_LIMIT = 200;
const MAX_PAGES = 10;

/** Groups invocations by UTC day and averages CPU, memory, and fee. */
export function aggregateResourceTrend(
  invocations: Invocation[]
): ResourceTrendPoint[] {
  const byDay = new Map<
    string,
    { cpu: number; mem: number; fee: number; count: number }
  >();

  for (const inv of invocations) {
    const date = inv.ledger_closed_at.slice(0, 10);
    const bucket = byDay.get(date) ?? { cpu: 0, mem: 0, fee: 0, count: 0 };
    bucket.cpu += inv.cpu_insn;
    bucket.mem += inv.mem_byte;
    bucket.fee += inv.resource_fee_charged;
    bucket.count += 1;
    byDay.set(date, bucket);
  }

  return [...byDay.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([date, b]) => ({
      date,
      avg_cpu_insn: b.cpu / b.count,
      avg_mem_byte: b.mem / b.count,
      avg_fee: b.fee / b.count,
      count: b.count,
    }));
}

/**
 * Fetches the last `days` days of invocations (paging through the endpoint)
 * and aggregates them into daily averages.
 */
export async function getResourceTrend(
  id: string,
  days = 30
): Promise<ResourceTrendPoint[]> {
  const since = new Date(Date.now() - days * DAY_MS).toISOString();
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

  return aggregateResourceTrend(invocations);
}
