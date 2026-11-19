"use client";

import Link from "next/link";
import useSWR from "swr";
import { EventVolumeChart } from "@/components/EventVolumeChart";
import { StatCard } from "@/components/StatCard";
import {
  getGlobalStats,
  getLiveActivity,
  listAlerts,
} from "@/lib/api";
import type {
  ContractAlert,
  LiveActivityResponse,
  VolumePoint,
} from "@/lib/types";

const ALERT_WINDOW_LIMIT = 5;
const HOT_CONTRACT_LIMIT = 5;

function formatNumber(value: number): string {
  return value.toLocaleString();
}

function shortId(id: string): string {
  return id.length > 12 ? `${id.slice(0, 6)}…${id.slice(-4)}` : id;
}

function WidgetMessage({ children }: { children: React.ReactNode }) {
  return (
    <p className="px-4 py-8 text-center text-sm text-[var(--color-text-secondary)]">
      {children}
    </p>
  );
}

export function StatsWidget() {
  const { data, error, isLoading } = useSWR(
    "dashboard:global-stats",
    getGlobalStats,
    { refreshInterval: 60_000 }
  );
  if (error) return <WidgetMessage>Could not load network stats.</WidgetMessage>;
  if (isLoading || !data) return <WidgetMessage>Loading stats…</WidgetMessage>;

  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <StatCard
        label="Contracts indexed"
        value={formatNumber(data.tracked_contracts)}
      />
      <StatCard label="Events" value={formatNumber(data.total_events)} />
      <StatCard
        label="Invocations"
        value={formatNumber(data.total_invocations)}
      />
      <StatCard
        label="Storage entries"
        value={formatNumber(data.total_storage_entries)}
      />
    </div>
  );
}

/** Sums every contract's per-minute buckets into one series. */
export function volumeSeries(activity: LiveActivityResponse): VolumePoint[] {
  const length = activity.contracts.reduce(
    (longest, contract) =>
      Math.max(longest, contract.per_minute?.length ?? 0),
    0
  );
  const start = new Date(activity.window_start).getTime();
  return Array.from({ length }, (_, index) => {
    const count = activity.contracts.reduce(
      (sum, contract) => sum + (contract.per_minute?.[index] ?? 0),
      0
    );
    const at = new Date(start + index * 60_000);
    return {
      date: at.toLocaleTimeString(undefined, {
        hour: "2-digit",
        minute: "2-digit",
        hour12: false,
      }),
      ledger: 0,
      count,
    };
  });
}

export function VolumeWidget() {
  const { data, error, isLoading } = useSWR(
    "dashboard:live-activity",
    () => getLiveActivity(60),
    { refreshInterval: 30_000 }
  );
  if (error) return <WidgetMessage>Could not load event volume.</WidgetMessage>;
  if (isLoading || !data) return <WidgetMessage>Loading volume…</WidgetMessage>;
  const series = volumeSeries(data);
  if (series.every((point) => point.count === 0)) {
    return <WidgetMessage>No events in the last hour.</WidgetMessage>;
  }
  return <EventVolumeChart data={series} />;
}

const SEVERITY_TONE: Record<ContractAlert["severity"], string> = {
  Critical: "text-[var(--color-danger)]",
  Warning: "text-[var(--color-warning)]",
  Info: "text-[var(--color-text-secondary)]",
};

export function AlertsWidget() {
  const { data, error, isLoading } = useSWR(
    "dashboard:recent-alerts",
    () => listAlerts(undefined, { limit: ALERT_WINDOW_LIMIT }),
    { refreshInterval: 30_000 }
  );
  if (error) return <WidgetMessage>Could not load alerts.</WidgetMessage>;
  if (isLoading || !data) return <WidgetMessage>Loading alerts…</WidgetMessage>;
  const alerts = data.alerts.slice(0, ALERT_WINDOW_LIMIT);
  if (alerts.length === 0) {
    return <WidgetMessage>No recent alerts.</WidgetMessage>;
  }

  return (
    <ul className="divide-y divide-[var(--color-border)]">
      {alerts.map((alert) => (
        <li
          key={`${alert.tx_hash}-${alert.severity}-${alert.ledger}`}
          className="flex items-start gap-3 px-4 py-3 text-sm"
        >
          <span
            className={`w-16 shrink-0 text-xs font-semibold ${SEVERITY_TONE[alert.severity]}`}
          >
            {alert.severity}
          </span>
          <div className="min-w-0 flex-1">
            <p className="truncate" title={alert.message}>
              {alert.message}
            </p>
            <Link
              href={`/contracts/${alert.contract_id}`}
              className="font-mono text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-accent)]"
            >
              {shortId(alert.contract_id)}
            </Link>
          </div>
        </li>
      ))}
    </ul>
  );
}

export function HotContractsWidget() {
  const { data, error, isLoading } = useSWR(
    "dashboard:hot-contracts",
    () => getLiveActivity(30),
    { refreshInterval: 30_000 }
  );
  if (error) {
    return <WidgetMessage>Could not load hot contracts.</WidgetMessage>;
  }
  if (isLoading || !data) return <WidgetMessage>Loading contracts…</WidgetMessage>;
  const contracts = data.contracts.slice(0, HOT_CONTRACT_LIMIT);
  if (contracts.length === 0) {
    return <WidgetMessage>No activity in the last 30 minutes.</WidgetMessage>;
  }

  return (
    <ol className="divide-y divide-[var(--color-border)]">
      {contracts.map((contract, index) => (
        <li
          key={contract.contract_id}
          className="flex items-center gap-3 px-4 py-2.5 text-sm"
        >
          <span className="w-5 shrink-0 text-right text-xs tabular-nums text-[var(--color-text-secondary)]">
            {index + 1}
          </span>
          <Link
            href={`/contracts/${contract.contract_id}`}
            className="min-w-0 flex-1 truncate hover:text-[var(--color-accent)]"
          >
            {contract.label || (
              <span className="font-mono">
                {shortId(contract.contract_id)}
              </span>
            )}
          </Link>
          <span className="shrink-0 text-xs tabular-nums text-[var(--color-text-secondary)]">
            {formatNumber(contract.total)} events
          </span>
        </li>
      ))}
    </ol>
  );
}
