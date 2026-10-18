"use client";

import Link from "next/link";
import { Fragment, useCallback, useEffect, useRef, useState } from "react";
import {
  getWatchdogStats,
  listAlertGroups,
  listMonitoredContracts,
  listAlerts,
} from "@/lib/api";
import type {
  AlertGroup,
  AlertSeverity,
  ContractAlert,
  MonitoredContract,
  WatchdogStats,
} from "@/lib/types";
import { StatCard } from "@/components/StatCard";
import { TableSkeleton } from "@/components/Skeleton";
import { ErrorState, errorMessage } from "@/components/ErrorState";
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

const SEVERITIES: AlertSeverity[] = ["Info", "Warning", "Critical"];

const selectClass =
  "rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-1.5 text-xs text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]";

function viewToggleClass(active: boolean): string {
  return `px-3 py-1.5 text-xs font-medium transition-colors ${
    active
      ? "bg-[var(--color-accent)] text-white"
      : "text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
  }`;
}

export default function WatchdogPage() {
  const { network } = useNetwork();
  const [stats, setStats] = useState<WatchdogStats>(ZERO_STATS);
  const [statsError, setStatsError] = useState<string | null>(null);
  const [statsReload, setStatsReload] = useState(0);
  const [contracts, setContracts] = useState<MonitoredContract[]>([]);
  const [contractsLoading, setContractsLoading] = useState(true);
  const [contractsError, setContractsError] = useState<string | null>(null);

  // Alerts feed: "grouped" (default) shows the deduplicated /api/v1/alerts
  // view, "flat" the raw per-event /api/v1/watchdog/alerts feed.
  const [view, setView] = useState<"grouped" | "flat">("grouped");
  const [severityFilter, setSeverityFilter] = useState<AlertSeverity | "">("");
  const [contractFilter, setContractFilter] = useState("");

  const [groups, setGroups] = useState<AlertGroup[] | null>(null);
  const [groupsCursor, setGroupsCursor] = useState("");
  const [alerts, setAlerts] = useState<ContractAlert[] | null>(null);
  const [alertsCursor, setAlertsCursor] = useState("");
  const [alertsLoading, setAlertsLoading] = useState(false);
  // A failed first-page load of the active feed (null = no failure), and a
  // counter that re-runs the load for the retry button.
  const [feedError, setFeedError] = useState<string | null>(null);
  const [feedReload, setFeedReload] = useState(0);
  // A failed "load more" keeps the rows already shown and offers a retry.
  const [moreError, setMoreError] = useState<string | null>(null);

  // Expandable group members: group_key -> raw alerts fetched on first expand.
  const [expandedKey, setExpandedKey] = useState<string | null>(null);
  const [members, setMembers] = useState<Record<string, ContractAlert[]>>({});
  const [membersLoading, setMembersLoading] = useState<string | null>(null);
  const [membersError, setMembersError] = useState<Record<string, string>>({});

  // Pagination state: stack of cursors, index 0 = first page. nextCursor is
  // the API's next_cursor for the current page ("" on the last page).
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const [cursorIndex, setCursorIndex] = useState(0);
  const [nextCursor, setNextCursor] = useState("");

  useEffect(() => {
    let cancelled = false;
    setStatsError(null);
    getWatchdogStats(networkFilter(network))
      .then((s) => {
        if (!cancelled) setStats(s);
      })
      .catch((err: unknown) => {
        // Zeros would read as "nothing monitored", so surface the failure.
        if (!cancelled)
          setStatsError(errorMessage(err, "The stats request failed."));
      });
    return () => {
      cancelled = true;
    };
  }, [network, statsReload]);

  // Load the first page of the active alerts feed whenever the network or a
  // filter changes. A sequence counter ensures only the newest request may
  // write to state, so a slow response can't overwrite a newer page.
  const feedSeq = useRef(0);
  useEffect(() => {
    const seq = ++feedSeq.current;
    const filter = networkFilter(network);
    const contractId = contractFilter || undefined;
    const severity = severityFilter || undefined;

    // Collapse members and reset paging: a new filter means a new feed.
    setExpandedKey(null);
    setMembers({});
    setMembersError({});
    setAlertsLoading(false);
    setFeedError(null);
    setMoreError(null);

    if (view === "grouped") {
      setGroups(null);
      setGroupsCursor("");
      listAlertGroups({
        contractId,
        severity,
        network: filter,
        limit: PAGE_SIZE,
      })
        .then((res) => {
          if (seq !== feedSeq.current) return;
          setGroups(res.groups ?? []);
          setGroupsCursor(res.next_cursor ?? "");
        })
        .catch((err: unknown) => {
          if (seq !== feedSeq.current) return;
          setGroups([]);
          setGroupsCursor("");
          setFeedError(errorMessage(err, "The alerts request failed."));
        });
      return;
    }

    setAlerts(null);
    setAlertsCursor("");
    listAlerts(contractId, { severity, network: filter, limit: PAGE_SIZE })
      .then((res) => {
        if (seq !== feedSeq.current) return;
        setAlerts(res.alerts ?? []);
        setAlertsCursor(res.next_cursor ?? "");
      })
      .catch((err: unknown) => {
        if (seq !== feedSeq.current) return;
        setAlerts([]);
        setAlertsCursor("");
        setFeedError(errorMessage(err, "The alerts request failed."));
      });
  }, [network, severityFilter, contractFilter, view, feedReload]);

  // Only the most recent loadContracts() may write to state, so a slow
  // response can't overwrite a newer page.
  const loadSeq = useRef(0);

  const loadContracts = useCallback(
    async (cursor: string | null) => {
      const seq = ++loadSeq.current;
      setContractsLoading(true);
      setContractsError(null);
      try {
        const data = await listMonitoredContracts({
          cursor: cursor ?? undefined,
          limit: PAGE_SIZE,
          network: networkFilter(network),
        });
        if (seq !== loadSeq.current) return;
        setContracts(data.contracts ?? []);
        setNextCursor(data.next_cursor ?? "");
      } catch (err) {
        if (seq !== loadSeq.current) return;
        setContracts([]);
        setNextCursor("");
        setContractsError(
          errorMessage(err, "The monitored contracts request failed.")
        );
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

  // Fetch the next grouped-alerts page and append it to the current feed.
  async function loadMoreGroups() {
    if (!groupsCursor || alertsLoading) return;
    setAlertsLoading(true);
    setMoreError(null);
    try {
      const res = await listAlertGroups({
        contractId: contractFilter || undefined,
        severity: severityFilter || undefined,
        network: networkFilter(network),
        cursor: groupsCursor,
        limit: PAGE_SIZE,
      });
      setGroups((prev) => [...(prev ?? []), ...(res.groups ?? [])]);
      setGroupsCursor(res.next_cursor ?? "");
    } catch (err) {
      // Keep the current feed on failure; the button stays available.
      setMoreError(errorMessage(err, "The next page failed to load."));
    } finally {
      setAlertsLoading(false);
    }
  }

  // Fetch the next flat alerts page and append it to the current feed.
  async function loadMoreAlerts() {
    if (!alertsCursor || alertsLoading) return;
    setAlertsLoading(true);
    setMoreError(null);
    try {
      const a = await listAlerts(contractFilter || undefined, {
        severity: severityFilter || undefined,
        network: networkFilter(network),
        limit: PAGE_SIZE,
        cursor: alertsCursor,
      });
      setAlerts((prev) => [...(prev ?? []), ...(a.alerts ?? [])]);
      setAlertsCursor(a.next_cursor ?? "");
    } catch (err) {
      // Keep the current feed on failure; the button stays available.
      setMoreError(errorMessage(err, "The next page failed to load."));
    } finally {
      setAlertsLoading(false);
    }
  }

  // Expand/collapse a group. The first expand fetches the group's raw member
  // alerts (the flat feed filtered to the group's contract + severity); they
  // are cached so re-expanding is instant.
  async function toggleGroup(group: AlertGroup) {
    if (expandedKey === group.group_key) {
      setExpandedKey(null);
      return;
    }
    setExpandedKey(group.group_key);
    if (members[group.group_key]) return;
    await loadMembers(group);
  }

  async function loadMembers(group: AlertGroup) {
    setMembersLoading(group.group_key);
    setMembersError((prev) => {
      const next = { ...prev };
      delete next[group.group_key];
      return next;
    });
    try {
      const res = await listAlerts(group.contract_id, {
        severity: group.severity,
        network: networkFilter(network),
        limit: PAGE_SIZE,
      });
      setMembers((prev) => ({ ...prev, [group.group_key]: res.alerts ?? [] }));
    } catch (err) {
      setMembersError((prev) => ({
        ...prev,
        [group.group_key]: errorMessage(
          err,
          "The member alerts failed to load."
        ),
      }));
    } finally {
      setMembersLoading(null);
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
        <Link
          href="/watchdog/reports"
          className="ml-4 inline-block text-sm text-[var(--color-accent)] hover:underline"
        >
          SLA reports →
        </Link>
      </div>

      {/* Summary cards: values default to 0 while loading, but a failed
          request shows an error instead of zeros. */}
      {statsError ? (
        <ErrorState
          title="Couldn't load watchdog stats."
          message={statsError}
          onRetry={() => setStatsReload((n) => n + 1)}
        />
      ) : (
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
      )}

      {/* Monitored contracts */}
      <section>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-xl font-semibold">Monitored contracts</h2>
        </div>
        {contractsLoading ? (
          <TableSkeleton />
        ) : contractsError ? (
          <ErrorState
            title="Couldn't load monitored contracts."
            message={contractsError}
            onRetry={() => loadContracts(cursors[cursorIndex])}
          />
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

      {/* Recent alerts: grouped by default via the deduplicated
          /api/v1/alerts feed; the flat per-event feed is one toggle away. */}
      <section>
        <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
          <h2 className="text-xl font-semibold">Recent alerts</h2>
          <div className="flex flex-wrap items-end gap-3">
            <div
              role="group"
              aria-label="Alerts view"
              className="flex self-end rounded-md border border-[var(--color-border)]"
            >
              <button
                type="button"
                onClick={() => setView("grouped")}
                aria-pressed={view === "grouped"}
                className={viewToggleClass(view === "grouped")}
              >
                Grouped
              </button>
              <button
                type="button"
                onClick={() => setView("flat")}
                aria-pressed={view === "flat"}
                className={viewToggleClass(view === "flat")}
              >
                Flat feed
              </button>
            </div>
            <label className="flex flex-col gap-1 text-xs text-[var(--color-text-secondary)]">
              Severity
              <select
                aria-label="Filter alerts by severity"
                value={severityFilter}
                onChange={(e) =>
                  setSeverityFilter(e.target.value as AlertSeverity | "")
                }
                className={selectClass}
              >
                <option value="">All severities</option>
                {SEVERITIES.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </label>
            <label className="flex flex-col gap-1 text-xs text-[var(--color-text-secondary)]">
              Contract
              <select
                aria-label="Filter alerts by contract"
                value={contractFilter}
                onChange={(e) => setContractFilter(e.target.value)}
                className={selectClass}
              >
                <option value="">All contracts</option>
                {contracts.map((c) => (
                  <option key={c.contract_id} value={c.contract_id}>
                    {c.name}
                  </option>
                ))}
              </select>
            </label>
          </div>
        </div>

        {feedError ? (
          <ErrorState
            title="Couldn't load alerts."
            message={feedError}
            onRetry={() => setFeedReload((n) => n + 1)}
          />
        ) : view === "grouped" ? (
          groups === null ? (
            <TableSkeleton />
          ) : groups.length === 0 ? (
            <EmptyState
              title="No alerts yet"
              body="Alerts pushed via report_alert on the watchdog contract are deduplicated and appear here as groups."
            />
          ) : (
            <div className="overflow-x-auto rounded-lg border border-[var(--color-border)]">
              <table className="min-w-full text-left text-sm">
                <thead className="bg-[var(--color-bg-card)] text-[var(--color-text-secondary)]">
                  <tr>
                    <th className="px-4 py-2 font-medium">Severity</th>
                    <th className="px-4 py-2 font-medium">Contract</th>
                    <th className="px-4 py-2 font-medium">Rule</th>
                    <th className="px-4 py-2 font-medium">Count</th>
                    <th className="px-4 py-2 font-medium">First seen</th>
                    <th className="px-4 py-2 font-medium">Last seen</th>
                    <th className="px-4 py-2 font-medium">
                      <span className="sr-only">Members</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {groups.map((g) => {
                    const open = expandedKey === g.group_key;
                    const groupMembers = members[g.group_key];
                    return (
                      <Fragment key={g.group_key}>
                        <tr className="border-t border-[var(--color-border)] transition-colors hover:bg-[var(--color-bg-card)]">
                          <td className="px-4 py-3">
                            <SeverityBadge severity={g.severity} />
                          </td>
                          <td className="px-4 py-3 font-mono text-xs text-[var(--color-text-secondary)]">
                            <Link
                              href={`/watchdog/${g.contract_id}`}
                              className="hover:text-[var(--color-accent)]"
                              title={g.contract_id}
                            >
                              {truncateMiddle(g.contract_id)}
                            </Link>
                          </td>
                          <td className="px-4 py-3 text-[var(--color-text-secondary)]">
                            {g.rule || "—"}
                          </td>
                          <td className="px-4 py-3 tabular-nums">{g.count}</td>
                          <td className="px-4 py-3 tabular-nums text-[var(--color-text-secondary)]">
                            {formatTime(g.first_seen)}
                          </td>
                          <td className="px-4 py-3 tabular-nums text-[var(--color-text-secondary)]">
                            {formatTime(g.last_seen)}
                          </td>
                          <td className="px-4 py-3 text-right">
                            <button
                              type="button"
                              onClick={() => toggleGroup(g)}
                              aria-expanded={open}
                              aria-controls={`alert-group-members-${g.id}`}
                              aria-label={
                                open
                                  ? `Collapse alert group ${g.id}`
                                  : `Expand alert group ${g.id}`
                              }
                              className="rounded-md border border-[var(--color-border)] px-2 py-1 text-xs text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
                            >
                              {open ? "▾" : "▸"}
                              <span className="ml-1">
                                {g.count} alert{g.count === 1 ? "" : "s"}
                              </span>
                            </button>
                          </td>
                        </tr>
                        {open && (
                          <tr id={`alert-group-members-${g.id}`}>
                            <td colSpan={7} className="px-4 py-3">
                              <div
                                data-testid="alert-group-members"
                                className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] p-3"
                              >
                                <p className="mb-2 text-xs text-[var(--color-text-secondary)]">
                                  Raw alerts for this contract and severity.
                                </p>
                                {membersLoading === g.group_key ? (
                                  <p
                                    role="status"
                                    aria-busy="true"
                                    className="text-sm text-[var(--color-text-secondary)]"
                                  >
                                    Loading members…
                                  </p>
                                ) : membersError[g.group_key] ? (
                                  <ErrorState
                                    compact
                                    title="Couldn't load the raw alerts."
                                    message={membersError[g.group_key]}
                                    onRetry={() => loadMembers(g)}
                                  />
                                ) : !groupMembers ||
                                  groupMembers.length === 0 ? (
                                  <p className="text-sm text-[var(--color-text-secondary)]">
                                    No raw alerts available for this group.
                                  </p>
                                ) : (
                                  <table className="min-w-full text-left text-xs">
                                    <thead className="text-[var(--color-text-secondary)]">
                                      <tr>
                                        <th className="px-2 py-1 font-medium">
                                          Message
                                        </th>
                                        <th className="px-2 py-1 font-medium">
                                          Ledger
                                        </th>
                                        <th className="px-2 py-1 font-medium">
                                          Transaction
                                        </th>
                                        <th className="px-2 py-1 font-medium">
                                          When
                                        </th>
                                      </tr>
                                    </thead>
                                    <tbody>
                                      {groupMembers.map((m) => (
                                        <tr
                                          key={`${m.tx_hash}-${m.contract_id}`}
                                          className="border-t border-[var(--color-border)]"
                                        >
                                          <td className="px-2 py-1.5">
                                            {m.message}
                                          </td>
                                          <td className="px-2 py-1.5 tabular-nums">
                                            {m.ledger}
                                          </td>
                                          <td
                                            className="px-2 py-1.5 font-mono"
                                            title={m.tx_hash}
                                          >
                                            {truncateMiddle(m.tx_hash)}
                                          </td>
                                          <td className="px-2 py-1.5 tabular-nums text-[var(--color-text-secondary)]">
                                            {formatTime(m.timestamp)}
                                          </td>
                                        </tr>
                                      ))}
                                    </tbody>
                                  </table>
                                )}
                              </div>
                            </td>
                          </tr>
                        )}
                      </Fragment>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )
        ) : alerts === null ? (
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

        {/* Load more (per view) */}
        {moreError && !feedError && (
          <ErrorState
            compact
            className="mt-4"
            title="Couldn't load more alerts."
            message={moreError}
          />
        )}
        {!feedError &&
          view === "grouped" &&
          groups !== null &&
          groups.length > 0 &&
          groupsCursor !== "" && (
            <div className="mt-4 text-center">
              <button
                type="button"
                onClick={loadMoreGroups}
                disabled={alertsLoading}
                aria-busy={alertsLoading}
                className="rounded-md border border-[var(--color-border)] px-4 py-2 text-sm transition-colors hover:bg-[var(--color-bg-card)] disabled:opacity-50"
              >
                {alertsLoading ? "Loading…" : "Load more groups"}
              </button>
            </div>
          )}
        {!feedError &&
          view === "flat" &&
          alerts !== null &&
          alertsCursor !== "" && (
            <div className="mt-4 text-center">
              <button
                type="button"
                onClick={loadMoreAlerts}
                disabled={alertsLoading}
                aria-busy={alertsLoading}
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
