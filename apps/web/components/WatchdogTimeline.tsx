"use client";

import { useMemo } from "react";
import type { HealthCheck, HealthStatus } from "@/lib/types";
import { HEALTH_STATUSES, healthColor } from "@/components/WatchdogBadges";

interface WatchdogTimelineProps {
  checks: HealthCheck[];
}

interface TimelineSegment {
  key: string;
  status: HealthStatus;
  color: string;
  weight: number;
  checks: HealthCheck[];
}

function timeOf(iso: string): number {
  return Date.parse(iso);
}

/**
 * Collapse the health-check history into consecutive same-status runs and size
 * each run by how long it lasted, so "healthy for two days, degraded for one"
 * reads at a glance. Falls back to equal widths when timestamps are missing.
 */
function buildSegments(checks: HealthCheck[]): TimelineSegment[] {
  // The API returns newest first; a timeline reads oldest -> newest.
  const ordered = [...checks].sort((a, b) => {
    const ta = timeOf(a.timestamp);
    const tb = timeOf(b.timestamp);
    if (Number.isNaN(ta) || Number.isNaN(tb)) return 0;
    return ta - tb;
  });

  const runs: HealthCheck[][] = [];
  for (const check of ordered) {
    const run = runs[runs.length - 1];
    if (run && run[0].status === check.status) run.push(check);
    else runs.push([check]);
  }

  const times = ordered
    .map((check) => timeOf(check.timestamp))
    .filter((t) => !Number.isNaN(t));
  const span = times.length > 1 ? Math.max(...times) - Math.min(...times) : 0;
  const averageGap =
    span > 0 && ordered.length > 1 ? span / (ordered.length - 1) : 1;

  return runs.map((run, index) => {
    const start = timeOf(run[0].timestamp);
    const next = runs[index + 1];
    const end = next
      ? timeOf(next[0].timestamp)
      : timeOf(run[run.length - 1].timestamp);
    const duration = end - start;
    return {
      key: `${run[0].tx_hash}-${index}`,
      status: run[0].status,
      color: healthColor(run[0].status),
      weight: Number.isFinite(duration) && duration > 0 ? duration : averageGap,
      checks: run,
    };
  });
}

function formatTimestamp(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? iso : date.toLocaleString();
}

/**
 * Horizontal timeline of watchdog status transitions. Each run of consecutive
 * same-status checks is one segment, coloured by status, and every change is
 * listed underneath with its timestamp and ledger.
 */
export function WatchdogTimeline({ checks }: WatchdogTimelineProps) {
  const { segments, transitions } = useMemo(() => {
    const built = buildSegments(checks);
    return {
      segments: built,
      transitions: built.slice(1).map((segment, index) => ({
        key: segment.key,
        from: built[index].status,
        to: segment.status,
        check: segment.checks[0],
      })),
    };
  }, [checks]);

  if (segments.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-[var(--color-border)] bg-[var(--color-bg-card)] p-6 text-center text-sm text-[var(--color-text-secondary)]">
        No health checks recorded yet.
      </p>
    );
  }

  const first = segments[0].checks[0];
  const lastSegment = segments[segments.length - 1];
  const last = lastSegment.checks[lastSegment.checks.length - 1];

  return (
    <figure className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4">
      <div
        data-testid="watchdog-timeline"
        role="img"
        aria-label={`Health status timeline for ${checks.length} checks across ${segments.length} status periods, oldest first`}
        className="flex h-10 w-full overflow-hidden rounded-md"
      >
        {segments.map((segment, index) => (
          <div
            key={segment.key}
            data-testid="timeline-segment"
            data-status={segment.status}
            title={`${segment.status} · ${segment.checks.length} check(s) · ${formatTimestamp(
              segment.checks[0].timestamp,
            )} → ${formatTimestamp(
              segment.checks[segment.checks.length - 1].timestamp,
            )}`}
            style={{
              flexGrow: segment.weight,
              flexBasis: 0,
              backgroundColor: segment.color,
              borderLeft:
                index > 0 ? "2px solid var(--color-bg-card)" : undefined,
            }}
          />
        ))}
      </div>

      <div className="mt-1 flex justify-between text-xs tabular-nums text-[var(--color-text-secondary)]">
        <span>{formatTimestamp(first.timestamp)}</span>
        <span>{formatTimestamp(last.timestamp)}</span>
      </div>

      <figcaption className="mt-3">
        <ul
          data-testid="timeline-legend"
          className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-[var(--color-text-secondary)]"
        >
          {HEALTH_STATUSES.map((status) => (
            <li
              key={status}
              data-testid="timeline-legend-item"
              data-status={status}
              className="inline-flex items-center gap-1.5"
            >
              <span
                aria-hidden="true"
                className="h-2.5 w-2.5 rounded-sm"
                style={{ backgroundColor: healthColor(status) }}
              />
              {status}
            </li>
          ))}
        </ul>
      </figcaption>

      {transitions.length > 0 && (
        <div className="mt-4 border-t border-[var(--color-border)] pt-3">
          <h3 className="text-xs font-medium uppercase tracking-wide text-[var(--color-text-secondary)]">
            {transitions.length}{" "}
            {transitions.length === 1
              ? "status transition"
              : "status transitions"}
          </h3>
          <ul className="mt-2 space-y-2">
            {transitions.map((transition) => (
              <li
                key={transition.key}
                data-testid="timeline-transition"
                className="flex flex-wrap items-center gap-2 text-xs"
              >
                <span className="inline-flex items-center gap-1.5 rounded-full border border-[var(--color-border)] px-2 py-0.5">
                  <span
                    aria-hidden="true"
                    className="h-1.5 w-1.5 rounded-full"
                    style={{ backgroundColor: healthColor(transition.from) }}
                  />
                  {transition.from}
                </span>
                <span
                  aria-hidden="true"
                  className="text-[var(--color-text-secondary)]"
                >
                  →
                </span>
                <span className="inline-flex items-center gap-1.5 rounded-full border border-[var(--color-border)] px-2 py-0.5">
                  <span
                    aria-hidden="true"
                    className="h-1.5 w-1.5 rounded-full"
                    style={{ backgroundColor: healthColor(transition.to) }}
                  />
                  {transition.to}
                </span>
                <span className="tabular-nums text-[var(--color-text-secondary)]">
                  {formatTimestamp(transition.check.timestamp)}
                </span>
                <span className="tabular-nums text-[var(--color-text-secondary)]">
                  ledger {transition.check.ledger}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </figure>
  );
}
