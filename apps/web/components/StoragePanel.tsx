"use client";

import { useState } from "react";
import type { StorageEntry } from "@/lib/types";

interface StoragePanelProps {
  entries: StorageEntry[];
  currentLedger: number;
  loading?: boolean;
  onLoadMore?: () => void;
  hasMore?: boolean;
}

function getUrgency(ledgersUntilExpiry: number | null): {
  severity: "safe" | "warning" | "danger";
  pct: number;
} {
  if (ledgersUntilExpiry == null) {
    return { severity: "safe", pct: 0 };
  }

  const MAX_THRESHOLD = 120960; // ~7 days at 5s/ledger
  const warningThreshold = 34560; // ~2 days
  const pct = Math.min(
    100,
    Math.max(0, ((MAX_THRESHOLD - ledgersUntilExpiry) / MAX_THRESHOLD) * 100),
  );

  if (ledgersUntilExpiry <= warningThreshold) {
    return { severity: "danger", pct };
  }
  if (ledgersUntilExpiry <= MAX_THRESHOLD) {
    return { severity: "warning", pct };
  }
  return { severity: "safe", pct };
}

const severityOrder = { danger: 0, warning: 1, safe: 2 };

function StorageRow({
  entry,
  currentLedger,
}: {
  entry: StorageEntry;
  currentLedger: number;
}) {
  const urgency = getUrgency(entry.ledgers_until_expiry);
  const barColor =
    urgency.severity === "danger"
      ? "bg-[var(--color-danger)]"
      : urgency.severity === "warning"
        ? "bg-[var(--color-warning)]"
        : "bg-[var(--color-safe)]";

  return (
    <tr className="border-b border-[var(--color-border)] transition-colors hover:bg-white/5">
      <td className="max-w-[180px] truncate px-4 py-3 font-mono text-xs">
        {entry.key_decoded || entry.key_xdr.slice(0, 24) + "..."}
      </td>
      <td className="px-4 py-3 text-sm capitalize">
        <span
          className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${
            entry.durability === "persistent"
              ? "bg-blue-900/40 text-blue-400"
              : entry.durability === "temporary"
                ? "bg-purple-900/40 text-purple-400"
                : "bg-yellow-900/40 text-yellow-400"
          }`}
        >
          {entry.durability}
        </span>
      </td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-2">
          <div className="h-2 w-full overflow-hidden rounded-full bg-white/10">
            <div
              className={`h-full rounded-full transition-all ${barColor}`}
              style={{ width: `${Math.min(100, urgency.pct)}%` }}
            />
          </div>
          <span
            className={`shrink-0 font-mono text-xs ${
              urgency.severity === "danger"
                ? "text-[var(--color-danger)]"
                : urgency.severity === "warning"
                  ? "text-[var(--color-warning)]"
                  : "text-[var(--color-text-secondary)]"
            }`}
          >
            {entry.ledgers_until_expiry != null
              ? entry.ledgers_until_expiry.toLocaleString()
              : "-"}
          </span>
        </div>
      </td>
      <td className="px-4 py-3 text-right font-mono text-xs text-[var(--color-text-secondary)]">
        {entry.last_modified_ledger != null
          ? (currentLedger - entry.last_modified_ledger).toLocaleString()
          : "-"}
      </td>
    </tr>
  );
}

export function StoragePanel({
  entries,
  currentLedger,
  loading,
  onLoadMore,
  hasMore,
}: StoragePanelProps) {
  const [durabilityFilter, setDurabilityFilter] = useState<string>("all");

  const sorted = [...entries]
    .filter(
      (e) =>
        durabilityFilter === "all" || e.durability === durabilityFilter,
    )
    .sort((a, b) => {
      const aOrder = severityOrder[getUrgency(a.ledgers_until_expiry).severity];
      const bOrder = severityOrder[getUrgency(b.ledgers_until_expiry).severity];
      return aOrder - bOrder;
    });

  if (!loading && entries.length === 0) {
    return (
      <div className="rounded-lg bg-[var(--color-bg-card)] p-8 text-center text-sm text-[var(--color-text-secondary)]">
        No storage entries found
      </div>
    );
  }

  return (
    <div className="rounded-lg bg-[var(--color-bg-card)]">
      <div className="flex items-center justify-between border-b border-[var(--color-border)] px-4 py-3">
        <div className="flex gap-2">
          {["all", "persistent", "temporary", "instance"].map((d) => (
            <button
              key={d}
              type="button"
              onClick={() => setDurabilityFilter(d)}
              className={`rounded-md px-2.5 py-1 text-xs font-medium capitalize transition-colors ${
                durabilityFilter === d
                  ? "bg-[var(--color-accent)] text-[var(--color-bg-page)]"
                  : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
              }`}
            >
              {d}
            </button>
          ))}
        </div>
        <div className="text-xs text-[var(--color-text-secondary)]">
          Current ledger: {currentLedger.toLocaleString()}
        </div>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-[var(--color-border)] text-left text-xs font-medium text-[var(--color-text-secondary)]">
              <th className="px-4 py-3">Key</th>
              <th className="px-4 py-3">Durability</th>
              <th className="px-4 py-3">TTL</th>
              <th className="px-4 py-3 text-right">Age (ledgers)</th>
            </tr>
          </thead>
          <tbody>
            {sorted.map((entry, i) => (
              <StorageRow
                key={`${entry.key_xdr}-${i}`}
                entry={entry}
                currentLedger={currentLedger}
              />
            ))}
          </tbody>
        </table>
      </div>

      {hasMore && (
        <div className="border-t border-[var(--color-border)] p-4 text-center">
          <button
            type="button"
            onClick={onLoadMore}
            disabled={loading}
            className="rounded-md bg-white/10 px-4 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:bg-white/20 disabled:opacity-50"
          >
            {loading ? "Loading..." : "Load More"}
          </button>
        </div>
      )}

      <div className="flex gap-4 border-t border-[var(--color-border)] px-4 py-2 text-xs text-[var(--color-text-secondary)]">
        <span className="flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-full bg-[var(--color-safe)]" />
          Safe (&gt;7d)
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-full bg-[var(--color-warning)]" />
          Warning (2-7d)
        </span>
        <span className="flex items-center gap-1">
          <span className="inline-block h-2 w-2 rounded-full bg-[var(--color-danger)]" />
          Danger (&lt;2d)
        </span>
      </div>
    </div>
  );
}
