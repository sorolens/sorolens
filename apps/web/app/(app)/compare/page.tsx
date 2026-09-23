"use client";

import { useEffect, useState } from "react";
import { listContractsAll, getContractStats, getMonitoredContract } from "@/lib/api";
import type { ContractSummary, MonitoredContract, ContractStatsApiResponse } from "@/lib/types";
import { CompareCard, ContractSelector } from "@/components/compare";

interface ComparisonItem {
  contract: ContractSummary;
  stats: ContractStatsApiResponse;
  stats7d: ContractStatsApiResponse;
  health_status: string;
  last_check: string | null;
}

export default function ComparePage() {
  const [contracts, setContracts] = useState<ContractSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [comparisonData, setComparisonData] = useState<ComparisonItem[]>([]);
  const [fetching, setFetching] = useState(false);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const data = await listContractsAll();
        if (!cancelled) setContracts(data.contracts ?? []);
      } catch {
        // Backend not reachable yet
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (selectedIds.length < 2) {
      setComparisonData([]);
      return;
    }

    let cancelled = false;
    async function fetchComparison() {
      setFetching(true);
      try {
        const results = await Promise.all(
          selectedIds.map(async (id) => {
            const [stats24h, rawStats7d, monitored] = await Promise.all([
              getContractStats(id, "24h").catch(() => null),
              getContractStats(id, "7d").catch(() => null),
              getMonitoredContract(id).catch(() => null),
            ]);

            const s24 = stats24h as ContractStatsApiResponse | null;
            const s7 = rawStats7d as ContractStatsApiResponse | null;
            const mon = monitored as MonitoredContract | null;

            const stats: ContractStatsApiResponse = {
              event_count: 0,
              invocation_count: 0,
              storage_count: 0,
              last_synced_ledger: 0,
              window_event_count: s24?.window_event_count ?? 0,
              window_invocation_count: s24?.window_invocation_count ?? 0,
              window_duration: s24?.window_duration ?? "",
            };

            const stats7d: ContractStatsApiResponse = {
              ...stats,
              window_event_count: s7?.window_event_count ?? 0,
              window_invocation_count: s7?.window_invocation_count ?? 0,
            };

            const health_status = mon?.status ?? "Healthy";
            const last_check = mon?.last_check ?? null;

            return {
              contract: contracts.find((c) => c.id === id)!,
              stats,
              stats7d,
              health_status,
              last_check,
            };
          }),
        );
        if (!cancelled) setComparisonData(results);
      } catch {
        // non-critical
      } finally {
        if (!cancelled) setFetching(false);
      }
    }
    fetchComparison();
    return () => { cancelled = true; };
  }, [selectedIds, contracts]);

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Compare Contracts</h1>
        <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
          Select 2–3 tracked contracts to compare their stats side by side
        </p>
      </div>

      <ContractSelector
        selected={selectedIds}
        onSelect={setSelectedIds}
        contracts={contracts}
      />

      {loading && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[1, 2, 3].map((i) => (
            <div
              key={i}
              className="h-64 animate-pulse rounded-xl bg-[var(--color-border)]"
            />
          ))}
        </div>
      )}

      {!loading && selectedIds.length < 2 && (
        <div className="rounded-lg border border-dashed border-[var(--color-border)] bg-[var(--color-bg-card)] px-8 py-16 text-center">
          <p className="text-lg font-medium text-[var(--color-text-primary)]">
            Select at least 2 contracts to compare
          </p>
          <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
            Choose from the tracked contracts list above
          </p>
        </div>
      )}

      {fetching && comparisonData.length > 0 && (
        <p className="text-sm text-[var(--color-text-secondary)]">Loading comparison data…</p>
      )}

      {!fetching && comparisonData.length >= 2 && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {comparisonData.map(({ contract, stats, stats7d, health_status, last_check }) => (
            <CompareCard
              key={contract.id}
              contract={contract}
              stats={{
                event_count_24h: stats.window_event_count ?? 0,
                event_count_7d: stats7d.window_event_count ?? 0,
                invocation_count: stats.window_invocation_count ?? 0,
                avg_cpu: 0,
                avg_fee: 0,
                last_activity: last_check,
              }}
              health_status={health_status}
              last_check={last_check}
            />
          ))}
        </div>
      )}
    </div>
  );
}
