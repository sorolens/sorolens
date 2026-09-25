import type { CompareContractEntry } from "./types";

/**
 * Comparison metric helpers for the /compare dashboard.
 *
 * Delta highlighting rule (issue #324): a metric that is more than 20% worse
 * than the best contract in the current comparison is amber (warning); more
 * than 50% worse is red (danger). "Worse" depends on the metric: higher
 * event/invocation counts and health scores are better, while higher average
 * CPU and fees are worse.
 */

export type DeltaTone = "neutral" | "warning" | "danger";

/** Ratio by which `value` is worse than `best` (0 when equal or better). */
export function worseRatio(value: number, best: number, higherIsBetter: boolean): number {
  if (!Number.isFinite(value) || !Number.isFinite(best) || best <= 0) return 0;
  const ratio = higherIsBetter ? (best - value) / best : (value - best) / best;
  return ratio > 0 ? ratio : 0;
}

/** Percentage (0–1) by which value is worse than best, or null when it is not. */
export function worsePercent(
  value: number | null,
  best: number | null,
  higherIsBetter: boolean,
): number | null {
  if (value === null || best === null) return null;
  const ratio = worseRatio(value, best, higherIsBetter);
  return ratio > 0 ? ratio : null;
}

/** Highlight tone for a metric value relative to the best in the comparison. */
export function deltaTone(
  value: number | null,
  best: number | null,
  higherIsBetter: boolean,
): DeltaTone {
  if (value === null || best === null) return "neutral";
  const ratio = worseRatio(value, best, higherIsBetter);
  if (ratio > 0.5) return "danger";
  if (ratio > 0.2) return "warning";
  return "neutral";
}

export interface CompareMetricSpec {
  key: string;
  label: string;
  higherIsBetter: boolean;
  /** Metric value for a contract, or null when it has nothing to show. */
  pick: (entry: CompareContractEntry) => number | null;
  format: (value: number) => string;
}

function formatInt(value: number): string {
  return Math.round(value).toLocaleString();
}

function formatDecimal(value: number): string {
  return value.toLocaleString(undefined, { maximumFractionDigits: 2 });
}

export const COMPARE_METRICS: CompareMetricSpec[] = [
  {
    key: "event_count",
    label: "Events",
    higherIsBetter: true,
    pick: (c) => c.event_count,
    format: formatInt,
  },
  {
    key: "invocation_count",
    label: "Invocations",
    higherIsBetter: true,
    pick: (c) => c.invocation_count,
    format: formatInt,
  },
  {
    key: "avg_cpu",
    label: "Avg CPU",
    higherIsBetter: false,
    pick: (c) => c.avg_cpu,
    format: formatDecimal,
  },
  {
    key: "avg_fee",
    label: "Avg Fee",
    higherIsBetter: false,
    pick: (c) => c.avg_fee,
    format: formatDecimal,
  },
  {
    key: "health_score",
    label: "Health Score",
    higherIsBetter: true,
    pick: (c) => c.health_score,
    format: formatInt,
  },
];

/**
 * Value a metric contributes for an entry: a contract with no indexed data
 * yet contributes nothing, so it can never be treated as the "best".
 */
export function metricValue(
  entry: CompareContractEntry,
  spec: CompareMetricSpec,
): number | null {
  if (!entry.has_data) return null;
  return spec.pick(entry);
}

/** Best value for a metric across the compared contracts, or null. */
export function bestFor(
  spec: CompareMetricSpec,
  entries: CompareContractEntry[],
): number | null {
  let best: number | null = null;
  for (const entry of entries) {
    const value = metricValue(entry, spec);
    if (value === null || !Number.isFinite(value)) continue;
    if (best === null || (spec.higherIsBetter ? value > best : value < best)) {
      best = value;
    }
  }
  return best;
}
