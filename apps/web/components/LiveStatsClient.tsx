"use client";

import { useCallback, useEffect, useState } from "react";
import { getGlobalStats, getWatchdogStats } from "@/lib/api";
import type { GlobalStats, WatchdogStats } from "@/lib/types";
import { ErrorState } from "@/components/ErrorState";
import { formatRelativeTime } from "@/lib/lastUpdated";

interface Props {
  className?: string;
  initialData: InitialData;
}

/** What the server half read, plus when it read it. */
interface InitialData {
  global: GlobalStats | null;
  watchdog: WatchdogStats | null;
  /**
   * `Date.now()` on the server. For an ISR-rendered landing page that is the
   * cache generation time, which is the only honest age for these figures —
   * without it the visitor cannot tell a cached number from a live one.
   */
  generatedAt?: number;
}

type Source = "global" | "watchdog";

interface Figure {
  label: string;
  source: Source;
  value: number | null;
}

/**
 * Past this age a figure is labelled as stale rather than presented as
 * current. The refresh happens on mount, so anything older than this is a
 * value the visitor is being asked to trust without a recent check.
 */
const STALE_AFTER_MS = 2 * 60_000;

/** Cadence for re-rendering the age labels so "5m ago" never freezes. */
const AGE_TICK_MS = 30_000;

export function LiveStatsClient({ className, initialData }: Props) {
  const [figures, setFigures] = useState<{
    global: GlobalStats | null;
    watchdog: WatchdogStats | null;
  }>({ global: initialData.global, watchdog: initialData.watchdog });
  // When each source was last read successfully. A source that never succeeded
  // has no timestamp, so its age is never rendered as "0m ago".
  const [stamps, setStamps] = useState<Record<Source, number | null>>({
    global: initialData.global ? (initialData.generatedAt ?? null) : null,
    watchdog: initialData.watchdog ? (initialData.generatedAt ?? null) : null,
  });
  // Sources whose most recent refresh attempt failed. Their values are the last
  // known ones, which is a different claim from "current".
  const [failed, setFailed] = useState<Record<Source, boolean>>({
    global: false,
    watchdog: false,
  });
  const [refreshing, setRefreshing] = useState(true);
  const [now, setNow] = useState<number | null>(null);
  const [reload, setReload] = useState(0);

  const retry = useCallback(() => {
    setReload((n) => n + 1);
  }, []);

  useEffect(() => {
    let cancelled = false;
    async function refresh() {
      setRefreshing(true);
      const [global, watchdog] = await Promise.allSettled([
        getGlobalStats(),
        getWatchdogStats(),
      ]);
      if (cancelled) return;
      setFailed({
        global: global.status === "rejected",
        watchdog: watchdog.status === "rejected",
      });
      setRefreshing(false);
      if (global.status === "rejected") {
        console.error("Unable to refresh global stats", global.reason);
      }
      if (watchdog.status === "rejected") {
        console.error("Unable to refresh watchdog stats", watchdog.reason);
      }
      const readAt = Date.now();
      setFigures((current) => ({
        global: global.status === "fulfilled" ? global.value : current.global,
        watchdog:
          watchdog.status === "fulfilled" ? watchdog.value : current.watchdog,
      }));
      setStamps((current) => ({
        global: global.status === "fulfilled" ? readAt : current.global,
        watchdog: watchdog.status === "fulfilled" ? readAt : current.watchdog,
      }));
    }
    refresh();
    return () => {
      cancelled = true;
    };
  }, [reload]);

  // The age labels are wall-clock state: computed after mount and ticked,
  // because a server render and its hydration would otherwise disagree.
  useEffect(() => {
    setNow(Date.now());
    const timer = setInterval(() => setNow(Date.now()), AGE_TICK_MS);
    return () => clearInterval(timer);
  }, []);

  const items: Figure[] = [
    {
      label: "Contracts indexed",
      source: "global",
      value: figures.global ? figures.global.tracked_contracts : null,
    },
    {
      label: "Events tracked",
      source: "global",
      value: figures.global ? figures.global.total_events : null,
    },
    {
      label: "Invocations traced",
      source: "global",
      value: figures.global ? figures.global.total_invocations : null,
    },
    {
      label: "Contracts monitored",
      source: "watchdog",
      value: figures.watchdog ? figures.watchdog.total_monitored : null,
    },
  ];

  function ageLabel(source: Source): string {
    const stamp = stamps[source];
    if (stamp === null || now === null) return "";
    return formatRelativeTime(stamp, now);
  }

  /** A figure is stale when its source could not be refreshed, or is old. */
  function isStale(source: Source): boolean {
    const stamp = stamps[source];
    if (failed[source]) return true;
    return stamp !== null && now !== null && now - stamp > STALE_AFTER_MS;
  }

  const hasAnyFigure = items.some((item) => item.value !== null);
  const nothingToLoad = !hasAnyFigure && !refreshing;
  const unavailable = nothingToLoad && (failed.global || failed.watchdog);
  const loading = !hasAnyFigure && !unavailable;
  const anyStale = hasAnyFigure && (isStale("global") || isStale("watchdog"));

  return (
    <div className={className}>
      {unavailable && (
        <ErrorState
          compact
          className="mb-3"
          title="Couldn't load live stats."
          message="The stats API did not answer, so no figures are shown. Nothing here is a real zero."
          onRetry={retry}
          retrying={refreshing}
        />
      )}
      {anyStale && (
        <div
          data-testid="live-stats-stale"
          className="mb-3 rounded-lg border border-[var(--color-warning-border)] bg-[var(--color-warning-bg)] p-4 text-sm"
        >
          <p className="font-medium text-[var(--color-warning-text)]">
            Some figures are out of date.
          </p>
          <p className="mt-1 text-[var(--color-text-secondary)]">
            {failed.global || failed.watchdog
              ? "The last refresh failed, so the tiles marked below are the last known values."
              : "The tiles marked below were read before this page was cached."}
          </p>
          <button
            type="button"
            onClick={retry}
            disabled={refreshing}
            className="mt-3 rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
          >
            {refreshing ? "Retrying…" : "Try again"}
          </button>
        </div>
      )}
      <div
        className="grid grid-cols-2 gap-3 sm:grid-cols-4"
        role={loading ? "status" : undefined}
        aria-busy={loading ? true : undefined}
        aria-label={loading ? "Loading live stats" : undefined}
      >
        {items.map((item) => {
          // A tile is marked when its value is old (or its source failed to
          // refresh), and a dash is marked too, so an absent figure is never
          // mistaken for "zero contracts indexed".
          const marked =
            item.value === null
              ? failed[item.source]
              : isStale(item.source) || failed[item.source];
          return (
            <div
              key={item.label}
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4 text-center"
            >
              {item.value === null && loading ? (
                <>
                  <span
                    aria-hidden="true"
                    className="mx-auto block h-8 w-16 animate-pulse rounded bg-[var(--color-bg-page)]"
                  />
                  <span className="sr-only">Loading {item.label}</span>
                </>
              ) : (
                <div className="text-2xl font-bold tabular-nums">
                  {item.value === null ? "—" : formatNum(item.value)}
                </div>
              )}
              <div className="mt-1 text-xs uppercase tracking-wider text-[var(--color-text-secondary)]">
                {item.label}
              </div>
              {marked && (
                <div
                  data-testid={`live-stats-age-${item.label}`}
                  className="mt-1 text-xs text-[var(--color-warning-text)]"
                >
                  {item.value === null
                    ? "unavailable"
                    : ageLabel(item.source) || "old"}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function formatNum(value: number): string {
  if (value < 1000) return String(value);
  if (value < 1_000_000) return `${(value / 1000).toFixed(1)}K`;
  return `${(value / 1_000_000).toFixed(1)}M`;
}
