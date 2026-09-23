"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import {
  getWatchdogStats,
  listMonitoredContracts,
  listAlerts,
} from "@/lib/api";
import type {
  ContractAlert,
  MonitoredContract,
  WatchdogStats,
} from "@/lib/types";
import { StatCard } from "@/components/StatCard";
import { TableSkeleton } from "@/components/Skeleton";
import { HealthBadge, SeverityBadge } from "@/components/WatchdogBadges";
import { networkFilter, useNetwork } from "@/lib/network";

const ZERO_STATS: WatchdogStats = {
  total_monitored: 0,
  healthy: 0,
  degraded: 0,
  unresponsive: 0,
  total_alerts: 0,
  critical_alerts: 0,
};

export default function WatchdogPage() {
  const { network } = useNetwork();
  const [stats, setStats] = useState<WatchdogStats>(ZERO_STATS);
  const [contracts, setContracts] = useState<MonitoredContract[] | null>(null);
  const [alerts, setAlerts] = useState<ContractAlert[] | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      const filter = networkFilter(network);
      const [s, c, a] = await Promise.all([
        getWatchdogStats(filter).catch(() => ZERO_STATS),
        listMonitoredContracts({ limit: 50, network: filter }).catch(() => ({ contracts: [], next_cursor: "" })),
        listAlerts(undefined, { limit: 20, network: filter }).catch(() => ({ alerts: [] })),
      ]);
      if (cancelled) return;
      setStats(s);
      setContracts(c.contracts ?? []);
      setAlerts(a.alerts ?? []);
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [network]);

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Watchdog</h1>
        <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
          On-chain health checks and alerts from the Sorolens watchdog Soroban
          contract. Register a contract by invoking{" "}
          <code className="rounded bg-[var(--color-bg-card)] px-1 py-0.5 text-xs">
            register_contract
          </code>{" "}
          on the deployed watchdog contract, then push status updates on your
          own schedule.
        </p>
      </div>

      {/* Summary cards: always render values, defaulting to 0 when the
          backend has nothing to report yet. */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <StatCard label="Monitored" value={stats.total_monitored} />
        <StatCard label="Healthy" value={stats.healthy} tone="safe" />
        <StatCard label="Degraded" value={stats.degraded} tone="warning" />
        <StatCard
          label="Critical alerts"
          value={stats.critical_alerts}
          tone="danger"
        />
      </div>

      {/* Monitored contracts */}
      <section>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-xl font-semibold">Monitored contracts</h2>
        </div>
        {contracts === null ? (
          <TableSkeleton />
        ) : contracts.length === 0 ? (
          <EmptyState
            title="No monitored contracts yet"
            body="Register a contract with the on-chain watchdog to get started. It will appear here on the indexer's next tick."
          />
        ) : (
          <div className="overflow-x-auto rounded-lg border border-[var(--color-border)]">
            <table className="min-w-full text-left text-sm">
              <thead className="bg-[var(--color-bg-card)] text-[var(--color-text-secondary)]">
                <tr>
                  <th className="px-4 py-2 font-medium">Name</th>
                  <th className="px-4 py-2 font-medium">Contract</th>
                  <th className="px-4 py-2 font-medium">Status</th>
                  <th className="px-4 py-2 font-medium">Interval</th>
                  <th className="px-4 py-2 font-medium">Last check</th>
                </tr>
              </thead>
              <tbody>
                {contracts.map((c) => (
                  <tr
                    key={c.contract_id}
                    className="border-t border-[var(--color-border)] transition-colors hover:bg-[var(--color-bg-card)]"
                  >
                    <td className="px-4 py-3 font-medium">
                      <Link
                        href={`/watchdog/${c.contract_id}`}
                        className="hover:text-[var(--color-accent)]"
                      >
                        {c.name}
                      </Link>
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-[var(--color-text-secondary)]">
                      {truncate(c.contract_id)}
                    </td>
                    <td className="px-4 py-3">
                      <HealthBadge status={c.status} />
                    </td>
                    <td className="px-4 py-3 tabular-nums">
                      {c.check_interval}s
                    </td>
                    <td className="px-4 py-3 tabular-nums text-[var(--color-text-secondary)]">
                      {c.last_check ? formatTime(c.last_check) : "never"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* Recent alerts */}
      <section>
        <h2 className="mb-4 text-xl font-semibold">Recent alerts</h2>
        {alerts === null ? (
          <TableSkeleton />
        ) : alerts.length === 0 ? (
          <EmptyState
            title="No alerts yet"
            body="Alerts pushed via report_alert on the watchdog contract will appear here."
          />
        ) : (
          <div className="overflow-x-auto rounded-lg border border-[var(--color-border)]">
            <table className="min-w-full text-left text-sm">
              <thead className="bg-[var(--color-bg-card)] text-[var(--color-text-secondary)]">
                <tr>
                  <th className="px-4 py-2 font-medium">Severity</th>
                  <th className="px-4 py-2 font-medium">Contract</th>
                  <th className="px-4 py-2 font-medium">Message</th>
                  <th className="px-4 py-2 font-medium">Ledger</th>
                  <th className="px-4 py-2 font-medium">When</th>
                </tr>
              </thead>
              <tbody>
                {alerts.map((a) => (
                  <tr
                    key={`${a.tx_hash}-${a.contract_id}`}
                    className="border-t border-[var(--color-border)]"
                  >
                    <td className="px-4 py-3">
                      <SeverityBadge severity={a.severity} />
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-[var(--color-text-secondary)]">
                      <Link
                        href={`/watchdog/${a.contract_id}`}
                        className="hover:text-[var(--color-accent)]"
                      >
                        {truncate(a.contract_id)}
                      </Link>
                    </td>
                    <td className="px-4 py-3">{a.message}</td>
                    <td className="px-4 py-3 tabular-nums">{a.ledger}</td>
                    <td className="px-4 py-3 tabular-nums text-[var(--color-text-secondary)]">
                      {formatTime(a.timestamp)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}

function EmptyState({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-lg border border-dashed border-[var(--color-border)] bg-[var(--color-bg-card)] p-8 text-center">
      <p className="font-medium">{title}</p>
      <p className="mt-1 text-sm text-[var(--color-text-secondary)]">{body}</p>
    </div>
  );
}

function truncate(id: string): string {
  if (id.length <= 16) return id;
  return `${id.slice(0, 8)}…${id.slice(-6)}`;
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}
