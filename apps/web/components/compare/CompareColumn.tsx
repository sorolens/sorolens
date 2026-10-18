"use client";

import type { CompareContractEntry } from "@/lib/types";
import {
  COMPARE_METRICS,
  bestFor,
  deltaTone,
  metricValue,
  worsePercent,
} from "@/lib/compare";
import { EventVolumeSparkline } from "./EventVolumeSparkline";

const networkTone: Record<string, string> = {
  testnet: "border-[var(--color-accent)] text-[var(--color-accent)]",
  mainnet: "border-[var(--color-safe)] text-[var(--color-safe)]",
  futurenet: "border-[var(--color-warning)] text-[var(--color-warning)]",
};

export function NetworkBadge({ network }: { network: string }) {
  const tone =
    networkTone[network] ??
    "border-[var(--color-border)] text-[var(--color-text-secondary)]";
  return (
    <span
      className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium capitalize ${tone}`}
    >
      {network || "unknown"}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  const tone =
    status === "active"
      ? "bg-green-900/40 text-green-400"
      : status === "error"
        ? "bg-red-900/40 text-red-400"
        : "bg-yellow-900/40 text-yellow-400";
  return (
    <span
      className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${tone}`}
    >
      {status || "unknown"}
    </span>
  );
}

interface CompareColumnProps {
  entry: CompareContractEntry;
  /** All compared entries, used to derive the best value per metric. */
  entries: CompareContractEntry[];
}

/**
 * One contract's column in the side-by-side comparison: identity, network and
 * status badges, an event-volume sparkline, and the metric rows with
 * amber/red highlighting when a metric is >20% / >50% worse than the best.
 */
export function CompareColumn({ entry, entries }: CompareColumnProps) {
  const label = entry.label || entry.id;

  return (
    <div className="flex flex-1 flex-col gap-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <h3 className="truncate text-base font-semibold text-[var(--color-text-primary)]">
            {label}
          </h3>
          <p
            className="mt-0.5 truncate font-mono text-[10px] text-[var(--color-text-secondary)]"
            title={entry.id}
          >
            {entry.id}
          </p>
        </div>
        <StatusBadge status={entry.status} />
      </div>

      <NetworkBadge network={entry.network} />

      <EventVolumeSparkline data={entry.event_volume} />

      {entry.error ? (
        <p className="text-xs text-[var(--color-danger)]" role="alert">
          {entry.error}
        </p>
      ) : !entry.has_data ? (
        <p className="text-xs text-[var(--color-text-secondary)]">
          No data yet for this contract.
        </p>
      ) : null}

      <dl className="space-y-2">
        {COMPARE_METRICS.map((spec) => {
          const value = metricValue(entry, spec);
          const best = bestFor(spec, entries);
          const tone = deltaTone(value, best, spec.higherIsBetter);
          const pct = worsePercent(value, best, spec.higherIsBetter);
          const toneClass =
            tone === "danger"
              ? "text-[var(--color-danger)]"
              : tone === "warning"
                ? "text-[var(--color-warning)]"
                : "text-[var(--color-text-primary)]";
          const badgeClass =
            tone === "danger"
              ? "bg-red-900/30"
              : tone === "warning"
                ? "bg-yellow-900/30"
                : "";
          return (
            <div
              key={spec.key}
              className="flex items-center justify-between gap-2 border-t border-[var(--color-border)] pt-2 first:border-t-0 first:pt-0"
            >
              <dt className="text-xs text-[var(--color-text-secondary)]">
                {spec.label}
              </dt>
              <dd className="text-right">
                <span className={`text-sm font-semibold ${toneClass}`}>
                  {value === null ? "—" : spec.format(value)}
                </span>
                {pct !== null && (
                  <span
                    className={`ml-2 rounded px-1 text-[10px] ${toneClass} ${badgeClass}`}
                    data-testid={`delta-${spec.key}`}
                  >
                    {Math.round(pct * 100)}% worse
                  </span>
                )}
              </dd>
            </div>
          );
        })}
      </dl>
    </div>
  );
}
