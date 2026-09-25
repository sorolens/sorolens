"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
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
import { truncateMiddle } from "@/lib/format";

const ZERO_STATS: WatchdogStats = {
  total_monitored: 0,
  healthy: 0,
  degraded: 0,
  unresponsive: 0,
  total_alerts: 0,
  critical_alerts: 0,
};

// Same page size as the /contracts list.
const PAGE_SIZE = 20;

export default function WatchdogPage() {
  const { network } = useNetwork();
  const [stats, setStats] = useState<WatchdogStats>(ZERO_STATS);
  const [contracts, setContracts] = useState<MonitoredContract[]>([]);
  const [contractsLoading, setContractsLoading] = useState(true);
  const [alerts, setAlerts] = useState<ContractAlert[] | null>(null);
  const [alertsCursor, setAlertsCursor] = useState("");
  const [alertsLoading, setAlertsLoading] = useState(false);

  // Pagination state: stack of cursors, index 0 = first page. nextCursor is
  // the API's next_cursor for the current page ("" on the last page).
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const [cursorIndex, setCursorIndex] = useState(0);
  const [nextCursor, setNextCursor] = useState("");

  useEffect(() => {
    let cancelled = false;
    async function load() {
      const filter = networkFilter(network);
      const [s, , a] = await Promise.all([
        getWatchdogStats(filter).catch(() => ZERO_STATS),
        listMonitoredContracts({ limit: 50, network: filter }).catch(() => ({
          contracts: [],
          next_cursor: "",
        })),
        listAlerts(undefined, { limit: 20, network: filter }).catch(() => ({
          alerts: [],
          next_cursor: "",
        })),
      ]);
      if (cancelled) return;
      setStats(s);
      setAlerts(a.alerts ?? []);
      setAlertsCursor(a.next_cursor ?? "");
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [network]);

  // Only the most recent loadContracts() may write to state, so a slow
  // response can't overwrite a newer page.
  const loadSeq = useRef(0);

  const loadContracts = useCallback(
    async (cursor: string | null) => {
      const seq = ++loadSeq.current;
      setContractsLoading(true);
      try {
        const data = await listMonitoredContracts({
          cursor: cursor ?? undefined,
          limit: PAGE_SIZE,
          network: networkFilter(network),
        });
        if (seq !== loadSeq.current) return;
        setContracts(data.contracts ?? []);
        setNextCursor(data.next_cursor ?? "");
      } catch {
        if (seq !== loadSeq.current) return;
        // Backend not reachable yet: show the empty state, not an error.
        setContracts([]);
        setNextCursor("");
      } finally {
        if (seq === loadSeq.current) setContractsLoading(false);
      }
    },
    [network]
  );

  useEffect(() => {
    loadContracts(cursors[cursorIndex]);
  }, [loadContracts, cursors, cursorIndex]);

  // Reset to the first page when the network filter changes. The ref guard
  // keeps this from firing an extra fetch on mount.
  const prevNetwork = useRef(network);
  useEffect(() => {
    if (prevNetwork.current !== network) {
      prevNetwork.current = network;
      setCursors([null]);
      setCursorIndex(0);
    }
  }, [network]);

  const handleNext = () => {
    if (!nextCursor) return;
    setCursors([...cursors.slice(0, cursorIndex + 1), nextCursor]);
    setCursorIndex(cursorIndex + 1);
  };

  const handlePrev = () => {
    if (cursorIndex === 0) return;
    setCursorIndex(cursorIndex - 1);
  };

  // Fetch the next alerts page and append it to the current feed.
  async function loadMoreAlerts() {
    if (!alertsCursor || alertsLoading) return;
    setAlertsLoading(true);
    try {
      const filter = networkFilter(network);
      const a = await listAlerts(undefined, {
        limit: 20,
        network: filter,
        cursor: alertsCursor,
      });
      setAlerts((prev) => [...(prev ?? []), ...(a.alerts ?? [])]);
      setAlertsCursor(a.next_cursor ?? "");
    } catch {
      // Keep the current feed on failure; the button stays available.
    } finally {
      setAlertsLoading(false);
    }
  }

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
        <Link
          href="/watchdog/notifications"
          className="mt-3 inline-block text-sm text-[var(--color-accent)] hover:underline"
        >
          Notification channels (Slack, Discord, PagerDuty) →
        </Link>
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
        {contractsLoading ? (
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
                    <td
                      title={c.contract_id}
                      className="px-4 py-3 font-mono text-xs text-[var(--color-text-secondary)]"
                    >
                      {truncateMiddle(c.contract_id)}
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

        {/* Pagination controls */}
        {!contractsLoading && contracts.length > 0 && (
          <div className="mt-4 flex items-center justify-between">
            <span className="text-xs text-[var(--color-text-secondary)]">
              Page {cursorIndex + 1}
            </span>
            <div className="flex gap-2">
              <button
                id="watchdog-contracts-prev-page"
                type="button"
                onClick={handlePrev}
                disabled={cursorIndex === 0}
                aria-label="Previous page"
                className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-40"
              >
                ← Previous
              </button>
              <button
                id="watchdog-contracts-next-page"
                type="button"
                onClick={handleNext}
                disabled={!nextCursor}
                aria-label="Next page"
                className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-40"
              >
                Next →
              </button>
            </div>
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
                        title={a.contract_id}
                      >
                        {truncateMiddle(a.contract_id)}
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
        {alerts !== null && alertsCursor !== "" && (
          <div className="mt-4 text-center">
            <button
              type="button"
              onClick={loadMoreAlerts}
              disabled={alertsLoading}
              className="rounded-md border border-[var(--color-border)] px-4 py-2 text-sm transition-colors hover:bg-[var(--color-bg-card)] disabled:opacity-50"
            >
              {alertsLoading ? "Loading…" : "Load more alerts"}
            </button>
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

function formatTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}
