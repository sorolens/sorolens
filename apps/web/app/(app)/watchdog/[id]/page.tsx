"use client";

import Link from "next/link";
import { use, useEffect, useState } from "react";
import { notFound } from "next/navigation";
import {
  ApiError,
  getMonitoredContract,
  getContractUptime,
  listAlerts,
  listHealthChecks,
} from "@/lib/api";
import type {
  ContractAlert,
  HealthCheck,
  MonitoredContract,
  UptimeWindow,
} from "@/lib/types";
import { CardSkeleton, TableSkeleton } from "@/components/Skeleton";
import { HealthBadge, SeverityBadge, UptimeBadge } from "@/components/WatchdogBadges";
import { WatchdogTimeline } from "@/components/WatchdogTimeline";

interface Props {
  params: Promise<{ id: string }>;
}

export default function MonitoredContractPage({ params }: Props) {
  const { id } = use(params);
  return <Content id={id} />;
}

/** The three uptime windows we display as badges. */
const UPTIME_WINDOWS: UptimeWindow[] = ["24h", "7d", "30d"];

function Content({ id }: { id: string }) {
  const [contract, setContract] = useState<MonitoredContract | null>(null);
  const [history, setHistory] = useState<HealthCheck[] | null>(null);
  const [alerts, setAlerts] = useState<ContractAlert[] | null>(null);
  const [notFoundError, setNotFoundError] = useState(false);
  const [unavailable, setUnavailable] = useState(false);

  // uptime: null = loading, number = resolved pct (0-100)
  const [uptime24h, setUptime24h] = useState<number | null>(null);
  const [uptime7d, setUptime7d] = useState<number | null>(null);
  const [uptime30d, setUptime30d] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const c = await getMonitoredContract(id);
        if (cancelled) return;
        setContract(c);
      } catch (e) {
        if (cancelled) return;
        if (e instanceof ApiError && e.status === 404) {
          setNotFoundError(true);
          return;
        }
        // Backend unreachable. Surface a friendly panel rather than a red
        // error box, since the visitor cannot act on a raw error string.
        setUnavailable(true);
        setHistory([]);
        setAlerts([]);
        return;
      }

      const [h, a] = await Promise.all([
        listHealthChecks(id, 50).catch(() => ({ health_checks: [] })),
        listAlerts(id, { limit: 50 }).catch(() => ({ alerts: [] })),
      ]);
      if (cancelled) return;
      setHistory(h.health_checks ?? []);
      setAlerts(a.alerts ?? []);

      // Fetch uptime badges in parallel; failures are swallowed so that a
      // missing uptime stat never breaks the rest of the detail page.
      const [u24, u7, u30] = await Promise.allSettled([
        getContractUptime(id, "24h"),
        getContractUptime(id, "7d"),
        getContractUptime(id, "30d"),
      ]);
      if (cancelled) return;
      if (u24.status === "fulfilled") setUptime24h(u24.value.uptime_pct);
      if (u7.status === "fulfilled") setUptime7d(u7.value.uptime_pct);
      if (u30.status === "fulfilled") setUptime30d(u30.value.uptime_pct);
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [id]);

  if (notFoundError) notFound();

  /** Map each window to its resolved pct (null while loading). */
  const uptimeByWindow: Record<UptimeWindow, number | null> = {
    "24h": uptime24h,
    "7d": uptime7d,
    "30d": uptime30d,
  };

  return (
    <div className="space-y-8">
      <div>
        <Link
          href="/watchdog"
          className="text-sm text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
        >
          ← Back to watchdog
        </Link>
      </div>

      {contract ? (
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-6">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold">{contract.name}</h1>
              <p className="mt-1 font-mono text-xs text-[var(--color-text-secondary)]">
                {contract.contract_id}
              </p>
            </div>
            <HealthBadge status={contract.status} />
          </div>
          <dl className="mt-6 grid grid-cols-2 gap-4 text-sm sm:grid-cols-4">
            <div>
              <dt className="text-[var(--color-text-secondary)]">Owner</dt>
              <dd className="mt-1 truncate font-mono text-xs">{contract.owner}</dd>
            </div>
            <div>
              <dt className="text-[var(--color-text-secondary)]">Check interval</dt>
              <dd className="mt-1 tabular-nums">{contract.check_interval}s</dd>
            </div>
            <div>
              <dt className="text-[var(--color-text-secondary)]">Last check</dt>
              <dd className="mt-1 tabular-nums">
                {contract.last_check
                  ? new Date(contract.last_check).toLocaleString()
                  : "never"}
              </dd>
            </div>
            <div>
              <dt className="text-[var(--color-text-secondary)]">Registered</dt>
              <dd className="mt-1 tabular-nums">
                {new Date(contract.registered_at).toLocaleString()}
              </dd>
            </div>
          </dl>

          {/* Uptime percentage badges */}
          <div className="mt-6 border-t border-[var(--color-border)] pt-4">
            <p className="mb-2 text-xs font-medium text-[var(--color-text-secondary)] uppercase tracking-wider">
              Uptime
            </p>
            <div
              className="flex flex-wrap gap-2"
              aria-label="Uptime percentage badges"
              data-testid="uptime-badges"
            >
              {UPTIME_WINDOWS.map((w) => (
                <UptimeBadge key={w} window={w} pct={uptimeByWindow[w]} />
              ))}
            </div>
          </div>
        </div>
      ) : unavailable ? (
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-8 py-16 text-center">
          <p className="text-lg font-medium text-[var(--color-text-primary)]">
            Watchdog data not available for this contract.
          </p>
          <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
            The contract may not be registered yet, or the indexer has not
            picked it up. Return to the{" "}
            <Link
              href="/watchdog"
              className="text-[var(--color-accent)] underline underline-offset-2 hover:opacity-80"
            >
              watchdog overview
            </Link>
            .
          </p>
        </div>
      ) : (
        <CardSkeleton />
      )}

      <section>
        <h2 className="mb-4 text-xl font-semibold">Health timeline</h2>
        {history === null ? (
          <TableSkeleton />
        ) : history.length === 0 ? (
          <p className="rounded-lg border border-dashed border-[var(--color-border)] bg-[var(--color-bg-card)] p-6 text-center text-sm text-[var(--color-text-secondary)]">
            No health checks recorded yet.
          </p>
        ) : (
          <WatchdogTimeline checks={history} />
        )}
      </section>

      <section>
        <h2 className="mb-4 text-xl font-semibold">Alerts</h2>
        {alerts === null ? (
          <TableSkeleton />
        ) : alerts.length === 0 ? (
          <p className="rounded-lg border border-dashed border-[var(--color-border)] bg-[var(--color-bg-card)] p-6 text-center text-sm text-[var(--color-text-secondary)]">
            No alerts have been raised for this contract.
          </p>
        ) : (
          <ul className="space-y-2">
            {alerts.map((a) => (
              <li
                key={`${a.tx_hash}-${a.contract_id}`}
                className="flex flex-wrap items-center gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-2 text-sm"
              >
                <SeverityBadge severity={a.severity} />
                <span>{a.message}</span>
                <span className="ml-auto tabular-nums text-[var(--color-text-secondary)]">
                  {new Date(a.timestamp).toLocaleString()}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
