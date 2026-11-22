import { getContractResourceTrend } from "@/lib/api";
import type { ResourceTrendPoint } from "@/lib/types";

/**
 * Daily resource-usage trend for one contract (issue #184).
 *
 * The API aggregates CPU, memory and fee into one bucket per UTC day, so the
 * chart loads in a single request (issue #600). This replaces the previous
 * implementation, which paged through up to 2,000 raw invocations on every
 * contract page load.
 */
export async function getResourceTrend(
  id: string,
  days = 30
): Promise<ResourceTrendPoint[]> {
  const response = await getContractResourceTrend(id, days);
  return response.points;
}
