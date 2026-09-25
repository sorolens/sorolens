"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  contractReportUrl,
  contractSLABadgeUrl,
  getContractReportHistory,
  listMonitoredContracts,
} from "@/lib/api";
import type { MonthlySLA } from "@/lib/types";
import { TableSkeleton } from "@/components/Skeleton";

const MONTHS_SHOWN = 12;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function currentMonth(): string {
  return new Date().toISOString().slice(0, 7);
}

function formatSeconds(seconds: number): string {
  if (!seconds || seconds <= 0) return "0s";
  if (seconds < 60) return `${Math.round(seconds)}s`;
  if (seconds < 3600) return `${(seconds / 60).toFixed(1)}m`;
  return `${(seconds / 3600).toFixed(1)}h`;
}

function formatPct(value: number): string {
  return `${value.toFixed(3)}%`;
}

/**
 * Uptime is the number the whole page is about, so it gets a colour scale
 * rather than a neutral chip: green at three nines, amber at risk, red missed.
 */
function uptimeTone(uptime: number, hasData: boolean): string {
  if (!hasData) return "text-[var(--color-text-secondary)]";
  if (uptime >= 99.9) return "text-green-400";
  if (uptime >= 99.0) return "text-yellow-400";
  return "text-red-400";
}

function verdict(m: MonthlySLA): string {
  if (!m.total_checks) return "No data";
  if (m.ongoing_outage) return "Degraded — incident open";
  if (m.uptime_pct >= 99.9) return "SLA met";
  if (m.uptime_pct >= 99.0) return "At risk";
  return "SLA missed";
}

// ---------------------------------------------------------------------------
// Components
// ---------------------------------------------------------------------------

function MetricCard({
  label,
  value,
  sub,
  tone,
}: {
  label: string;
  value: string;
  sub?: string;
  tone?: string;
}) {
  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4">
      <p className="text-xs uppercase tracking-wide text-[var(--color-text-secondary)]">
        {label}
      </p>
      <p
        className={`mt-1 text-2xl font-semibold ${tone ?? "text-[var(--color-text-primary)]"}`}
      >
        {value}
      </p>
      {sub && (
        <p className="mt-0.5 text-xs text-[var(--color-text-secondary)]">
          {sub}
        </p>
      )}
    </div>
  );
}

/**
 * Monthly uptime bar chart.
 *
 * Hand-rolled SVG rather than a charting dependency: the series is twelve
 * values against a fixed 0-100 axis, which is a few rects, and this keeps the
 * page's bundle unchanged.
 */
function UptimeChart({ months }: { months: MonthlySLA[] }) {
  const width = 720;
  const height = 180;
  const padX = 36;
  const padY = 20;
  const barGap = 6;

  const usable = width - padX * 2;
  const barWidth = months.length
    ? Math.max(4, usable / months.length - barGap)
    : 0;

  // A tight 90-100% window would exaggerate noise; 0-100 keeps the bars honest
  // about what 99.9% actually looks like next to a bad month.
  const scale = (pct: number) =>
    (height - padY * 2) * (Math.max(0, Math.min(100, pct)) / 100);

  return (
    <svg
      viewBox={`0 0 ${width} ${height + 24}`}
      className="w-full"
      role="img"
      aria-label="Monthly uptime for the last twelve months"
    >
      {[0, 25, 50, 75, 100].map((tick) => {
        const y = padY + (height - padY * 2) * (1 - tick / 100);
        return (
          <g key={tick}>
            <line
              x1={padX}
              x2={width - padX}
              y1={y}
              y2={y}
              stroke="var(--color-border)"
              strokeWidth="1"
            />
            <text
              x={padX - 6}
              y={y + 3}
              textAnchor="end"
              className="fill-[var(--color-text-secondary)]"
              fontSize="9"
            >
              {tick}%
            </text>
          </g>
        );
      })}

      {months.map((m, i) => {
        const hasData = m.total_checks > 0;
        const h = hasData ? Math.max(2, scale(m.uptime_pct)) : 0;
        const x = padX + i * (barWidth + barGap);
        const colour = !hasData
          ? "var(--color-border)"
          : m.uptime_pct >= 99.9
            ? "#2ea44f"
            : m.uptime_pct >= 99.0
              ? "#d29922"
              : "#d73a49";
        return (
          <g key={m.month}>
            <rect
              x={x}
              y={height - padY - h}
              width={barWidth}
              height={h}
              fill={colour}
              rx="2"
            >
              <title>
                {m.month}: {hasData ? formatPct(m.uptime_pct) : "no data"}
              </title>
            </rect>
            <text
              x={x + barWidth / 2}
              y={height + 8}
              textAnchor="middle"
              className="fill-[var(--color-text-secondary)]"
              fontSize="9"
            >
              {m.month.slice(5)}
            </text>
          </g>
        );
      })}
    </svg>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function SLAReportsPage() {
  const [contractId, setContractId] = useState("");
  const [month, setMonth] = useState(currentMonth);
  const [history, setHistory] = useState<MonthlySLA[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // The contract picker. A failure here is not fatal: the page still works for
  // a contract id typed or linked in directly.
  const [contracts, setContracts] = useState<
    { contract_id: string; name: string }[]
  >([]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const data = await listMonitoredContracts({ limit: 200 });
        if (cancelled) return;
        setContracts(data.contracts ?? []);
      } catch {
        if (!cancelled) setContracts([]);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const load = useCallback(async (id: string) => {
    if (!id) {
      setHistory([]);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const data = await getContractReportHistory(id, MONTHS_SHOWN);
      setHistory(data.months ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load report");
      setHistory([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(contractId);
  }, [contractId, load]);

  // The selected month's bucket, so the cards and the exports always agree with
  // the month the user picked rather than always showing the newest one.
  const selected = useMemo(
    () => history.find((m) => m.month === month) ?? null,
    [history, month]
  );

  const badgeUrl = contractId ? contractSLABadgeUrl(contractId, month) : "";

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">
          SLA &amp; uptime reports
        </h1>
        <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
          Monthly availability, incident response and exportable evidence per
          contract.
        </p>
      </div>

      {/* Controls */}
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-end">
        <div className="flex-1">
          <label
            htmlFor="sla-contract"
            className="mb-1.5 block text-sm font-medium text-[var(--color-text-secondary)]"
          >
            Contract
          </label>
          {contracts.length > 0 ? (
            <select
              id="sla-contract"
              value={contractId}
              onChange={(e) => setContractId(e.target.value)}
              className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2.5 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none"
            >
              <option value="">Select a monitored contract…</option>
              {contracts.map((c) => (
                <option key={c.contract_id} value={c.contract_id}>
                  {c.name ? `${c.name} — ${c.contract_id}` : c.contract_id}
                </option>
              ))}
            </select>
          ) : (
            <input
              id="sla-contract"
              type="text"
              value={contractId}
              onChange={(e) => setContractId(e.target.value.trim())}
              placeholder="C…"
              spellCheck={false}
              className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2.5 font-mono text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
            />
          )}
        </div>

        <div>
          <label
            htmlFor="sla-month"
            className="mb-1.5 block text-sm font-medium text-[var(--color-text-secondary)]"
          >
            Month
          </label>
          <input
            id="sla-month"
            type="month"
            value={month}
            onChange={(e) => setMonth(e.target.value || currentMonth())}
            className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2.5 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
        </div>

        <div className="flex gap-2">
          <a
            id="sla-export-csv"
            href={
              contractId
                ? contractReportUrl(contractId, month, "csv")
                : undefined
            }
            aria-disabled={!contractId}
            className={`rounded-lg border border-[var(--color-border)] px-4 py-2.5 text-sm font-medium transition-colors ${
              contractId
                ? "text-[var(--color-text-secondary)] hover:border-[var(--color-accent)] hover:text-[var(--color-text-primary)]"
                : "pointer-events-none opacity-40"
            }`}
          >
            Export CSV
          </a>
          <a
            id="sla-export-pdf"
            href={
              contractId
                ? contractReportUrl(contractId, month, "pdf")
                : undefined
            }
            aria-disabled={!contractId}
            className={`rounded-lg bg-[var(--color-accent)] px-4 py-2.5 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity ${
              contractId ? "hover:opacity-90" : "pointer-events-none opacity-40"
            }`}
          >
            Export PDF
          </a>
        </div>
      </div>

      {error && (
        <p className="mb-4 text-sm text-red-400" role="alert">
          {error}
        </p>
      )}

      {!contractId && (
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-8 py-16 text-center">
          <p className="text-lg font-medium text-[var(--color-text-primary)]">
            Choose a contract to see its SLA report
          </p>
          <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
            Reports are built from watchdog health checks and alerts, so a
            contract only appears once it is monitored.
          </p>
        </div>
      )}

      {contractId && loading && <TableSkeleton rows={6} />}

      {contractId && !loading && selected && (
        <>
          <div className="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <MetricCard
              label="Uptime"
              value={
                selected.total_checks ? formatPct(selected.uptime_pct) : "—"
              }
              sub={
                selected.total_checks
                  ? `${selected.healthy_checks}/${selected.total_checks} healthy checks`
                  : "no checks this month"
              }
              tone={uptimeTone(selected.uptime_pct, selected.total_checks > 0)}
            />
            <MetricCard
              label="Incidents"
              value={String(selected.incidents)}
              sub={selected.ongoing_outage ? "one still open" : "all recovered"}
            />
            <MetricCard
              label="MTTR"
              value={
                selected.incidents ? formatSeconds(selected.mttr_seconds) : "—"
              }
              sub={
                selected.ongoing_outage
                  ? "excludes the open incident"
                  : "mean time to recovery"
              }
            />
            <MetricCard
              label="Downtime"
              value={formatSeconds(selected.total_downtime_seconds)}
              sub={`longest ${formatSeconds(selected.longest_outage_seconds)}`}
            />
          </div>

          <div className="mb-6 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4">
            <div className="mb-2 flex flex-wrap items-baseline justify-between gap-2">
              <h2 className="text-sm font-semibold text-[var(--color-text-primary)]">
                Uptime — last {MONTHS_SHOWN} months
              </h2>
              <span className="text-xs text-[var(--color-text-secondary)]">
                {selected.month}: {verdict(selected)} · {selected.total_alerts}{" "}
                alert
                {selected.total_alerts === 1 ? "" : "s"}
              </span>
            </div>
            <UptimeChart months={history} />
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4">
              <h2 className="mb-3 text-sm font-semibold text-[var(--color-text-primary)]">
                Alerts in {selected.month}
              </h2>
              <dl className="space-y-1.5 text-sm">
                {[
                  ["Critical", selected.critical_alerts],
                  ["Warning", selected.warning_alerts],
                  ["Info", selected.info_alerts],
                  ["Total", selected.total_alerts],
                ].map(([label, value]) => (
                  <div key={String(label)} className="flex justify-between">
                    <dt className="text-[var(--color-text-secondary)]">
                      {label}
                    </dt>
                    <dd className="font-medium text-[var(--color-text-primary)]">
                      {String(value)}
                    </dd>
                  </div>
                ))}
              </dl>
              <p className="mt-3 border-t border-[var(--color-border)] pt-3 text-xs text-[var(--color-text-secondary)]">
                Coverage:{" "}
                {selected.first_check
                  ? `${new Date(selected.first_check).toLocaleString()} → ${
                      selected.last_check
                        ? new Date(selected.last_check).toLocaleString()
                        : "—"
                    }`
                  : "no checks recorded"}
              </p>
            </div>

            <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4">
              <h2 className="mb-3 text-sm font-semibold text-[var(--color-text-primary)]">
                Embed this month&apos;s badge
              </h2>
              {/* A live preview: the URL is the deliverable, so show the result. */}
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={badgeUrl}
                alt={`SLA badge for ${selected.month}`}
                height={20}
                className="mb-3"
              />
              <label
                htmlFor="sla-badge-snippet"
                className="mb-1 block text-xs text-[var(--color-text-secondary)]"
              >
                Markdown
              </label>
              <textarea
                id="sla-badge-snippet"
                readOnly
                rows={2}
                value={`![SLA](${badgeUrl})`}
                className="w-full resize-none rounded-lg border border-[var(--color-border)] bg-black/30 p-2 font-mono text-xs text-[var(--color-text-primary)]"
              />
              <p className="mt-3 border-t border-[var(--color-border)] pt-3 text-xs text-[var(--color-text-secondary)]">
                Exports are signed with HMAC-SHA256; the signature is returned
                in the <code className="font-mono">X-Report-Signature</code>{" "}
                header and embedded in the PDF.
              </p>
            </div>
          </div>
        </>
      )}

      {contractId && !loading && !selected && !error && (
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-8 py-12 text-center text-sm text-[var(--color-text-secondary)]">
          No report for {month}. The contract may not have been monitored then.
        </div>
      )}
    </div>
  );
}
