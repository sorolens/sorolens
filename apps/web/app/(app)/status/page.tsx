"use client";

import { useCallback, useEffect, useState } from "react";
import {
  getAPIHealth,
  getAPIReadiness,
  getAPIVersion,
  getIndexerStatus,
} from "@/lib/api";
import type {
  APIHealthResponse,
  APIVersionResponse,
  IndexerStatusResponse,
} from "@/lib/types";

interface Readiness {
  status: string;
  checks?: Record<string, string>;
}

function StateValue({ value }: { value: string | null }) {
  const healthy = value === "ok" || value === "connected";
  return (
    <span className={healthy ? "text-green-400" : "text-amber-400"}>
      {value ?? "Unavailable"}
    </span>
  );
}

function StatusSection({
  title,
  children,
  unavailable,
}: {
  title: string;
  children: React.ReactNode;
  unavailable: boolean;
}) {
  return (
    <section className="border-t border-[var(--color-border)] py-5">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h2 className="text-lg font-semibold">{title}</h2>
        {unavailable && (
          <span className="text-xs text-amber-400">Some data unavailable</span>
        )}
      </div>
      {children}
    </section>
  );
}

export default function StatusPage() {
  const [health, setHealth] = useState<APIHealthResponse | null>(null);
  const [readiness, setReadiness] = useState<Readiness | null>(null);
  const [version, setVersion] = useState<APIVersionResponse | null>(null);
  const [indexer, setIndexer] = useState<IndexerStatusResponse | null>(null);
  const [failures, setFailures] = useState<string[]>([]);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    const results = await Promise.allSettled([
      getAPIHealth(),
      getAPIReadiness(),
      getAPIVersion(),
      getIndexerStatus(),
    ]);
    const failed: string[] = [];
    if (results[0].status === "fulfilled") setHealth(results[0].value);
    else failed.push("API health");
    if (results[1].status === "fulfilled") setReadiness(results[1].value);
    else {
      setReadiness({ status: "unavailable" });
      failed.push("Readiness");
    }
    if (results[2].status === "fulfilled") setVersion(results[2].value);
    else failed.push("API version");
    if (results[3].status === "fulfilled") setIndexer(results[3].value);
    else failed.push("Indexer metrics");
    setFailures(failed);
    setLastChecked(new Date());
    setLoading(false);
  }, []);

  useEffect(() => {
    void refresh();
    const timer = window.setInterval(() => void refresh(), 30_000);
    return () => window.clearInterval(timer);
  }, [refresh]);

  return (
    <div>
      <header className="mb-5 flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="mb-1 text-xs font-medium uppercase text-[var(--color-text-secondary)]">
            Sorolens
          </p>
          <h1 className="text-2xl font-semibold">System status</h1>
          <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
            API availability and indexing progress by network.
          </p>
        </div>
        <div className="flex items-center gap-3">
          {lastChecked && (
            <span className="text-xs text-[var(--color-text-secondary)]">
              Updated {lastChecked.toLocaleTimeString()}
            </span>
          )}
          <button
            type="button"
            onClick={() => void refresh()}
            disabled={loading}
            className="rounded-md border border-[var(--color-border)] px-3 py-2 text-sm hover:border-[var(--color-accent)] disabled:opacity-50"
          >
            Refresh
          </button>
        </div>
      </header>

      {failures.length > 0 && (
        <p className="mb-4 text-sm text-amber-400" role="status">
          Partial status data: {failures.join(", ")} could not be reached.
        </p>
      )}

      <StatusSection title="API health" unavailable={!health}>
        {loading && !health ? (
          <p
            className="text-sm text-[var(--color-text-secondary)]"
            role="status"
          >
            Checking API health…
          </p>
        ) : health ? (
          <dl className="grid gap-4 sm:grid-cols-3">
            <div>
              <dt className="text-xs text-[var(--color-text-secondary)]">
                Service
              </dt>
              <dd className="mt-1">
                <StateValue value={health.status} />
              </dd>
            </div>
            <div>
              <dt className="text-xs text-[var(--color-text-secondary)]">
                Postgres
              </dt>
              <dd className="mt-1">
                <StateValue value={health.db} />
              </dd>
            </div>
            <div>
              <dt className="text-xs text-[var(--color-text-secondary)]">
                Redis
              </dt>
              <dd className="mt-1">
                <StateValue value={health.redis} />
              </dd>
            </div>
          </dl>
        ) : (
          <p className="text-sm text-amber-400">API health is unavailable.</p>
        )}
      </StatusSection>

      <StatusSection
        title="Readiness"
        unavailable={!readiness || readiness.status !== "ok"}
      >
        <p className="text-sm">
          <StateValue value={readiness?.status ?? null} />
        </p>
        {readiness?.checks && Object.keys(readiness.checks).length > 0 && (
          <dl className="mt-3 grid gap-3 sm:grid-cols-2">
            {Object.entries(readiness.checks).map(([name, state]) => (
              <div key={name}>
                <dt className="text-xs text-[var(--color-text-secondary)]">
                  {name}
                </dt>
                <dd className="mt-1">
                  <StateValue value={state} />
                </dd>
              </div>
            ))}
          </dl>
        )}
      </StatusSection>

      <StatusSection title="API version" unavailable={!version}>
        {version ? (
          <dl className="grid gap-4 sm:grid-cols-3">
            <div>
              <dt className="text-xs text-[var(--color-text-secondary)]">
                Version
              </dt>
              <dd className="mt-1 font-mono text-sm">{version.version}</dd>
            </div>
            <div>
              <dt className="text-xs text-[var(--color-text-secondary)]">
                Git SHA
              </dt>
              <dd className="mt-1 font-mono text-sm">{version.git_sha}</dd>
            </div>
            <div>
              <dt className="text-xs text-[var(--color-text-secondary)]">
                Built at
              </dt>
              <dd className="mt-1 text-sm">{version.built_at}</dd>
            </div>
          </dl>
        ) : (
          <p className="text-sm text-amber-400">
            Build metadata is unavailable.
          </p>
        )}
      </StatusSection>

      <StatusSection title="Indexer progress" unavailable={!indexer}>
        {!indexer ? (
          <p className="text-sm text-amber-400">
            Indexer metrics are unavailable.
          </p>
        ) : indexer.networks.length === 0 ? (
          <p className="text-sm text-[var(--color-text-secondary)]">
            No network metrics have been reported yet.
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="text-xs text-[var(--color-text-secondary)]">
                <tr>
                  <th className="py-2 pr-4 font-medium">Network</th>
                  <th className="py-2 pr-4 font-medium">Last indexed ledger</th>
                  <th className="py-2 font-medium">Lag (ledgers)</th>
                </tr>
              </thead>
              <tbody>
                {indexer.networks.map((network) => (
                  <tr
                    key={network.network}
                    className="border-t border-[var(--color-border)]"
                  >
                    <th
                      scope="row"
                      className="py-3 pr-4 font-medium capitalize"
                    >
                      {network.network}
                    </th>
                    <td className="py-3 pr-4 tabular-nums">
                      {network.last_indexed_ledger?.toLocaleString() ?? "—"}
                    </td>
                    <td className="py-3 tabular-nums">
                      {network.lag_ledgers?.toLocaleString() ?? "—"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </StatusSection>
    </div>
  );
}
