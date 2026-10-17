import { getContractResourceTrend } from "@/lib/api";
import type { ResourceTrendPoint } from "@/lib/types";

/**
 * One day of averaged per-invocation resource usage (issue #184).
 *
 * The chart is fed from the server-side /api/v1/contracts/{id}/resource-trend
 * endpoint in a single request (issue #600). Aggregation happens server-side,
 * so this is a pass-through: no client-side paging over /invocations and no
 * client-side aggregation, which previously fetched up to 2,000 invocations on
 * every contract page load.
 */
export async function getResourceTrend(
  id: string,
  days = 30
): Promise<ResourceTrendPoint[]> {
  return getContractResourceTrend(id, days);
}
