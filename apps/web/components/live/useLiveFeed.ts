"use client";

/**
 * Live data feed for the /live dashboard (issue #139).
 *
 * The API exposes two read endpoints that together cover everything the wall
 * display needs:
 *
 *   GET /api/v1/events/recent   -> the newest events across all contracts
 *   GET /api/v1/stats/activity  -> per-contract events-per-minute buckets
 *
 * Both are polled on a fixed cadence (5s by default). Polling rather than SSE
 * is deliberate and matches the rest of the dashboard: the API runs on Vercel
 * serverless functions, which cannot hold a long-lived stream (see
 * ARCHITECTURE.md §5.2).
 *
 * The hook hides the two things a long-running display cares about:
 *
 *  - The ticker must update in place, never re-render the page. New events are
 *    merged into the existing list and de-duplicated by id, so a poll that
 *    returns events already on screen changes nothing.
 *  - A browser tab left open on a monitor for days must not poll while hidden.
 *    Polling pauses when the document is hidden and resumes immediately when
 *    it becomes visible again.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { getLiveActivity, getRecentEvents } from "@/lib/api";
import type { ContractEvent, ContractEventRate } from "@/lib/types";

/** How many events the ticker holds, per the issue. */
export const TICKER_SIZE = 50;

/** Auto-refresh cadence, per the issue. */
export const REFRESH_MS = 5000;

/** Sparkline window: 30 one-minute buckets. */
export const ACTIVITY_MINUTES = 30;

/** How long a newly arrived event stays highlighted. */
const NEW_HIGHLIGHT_MS = 4000;

export interface LiveFeed {
  events: ContractEvent[];
  contracts: ContractEventRate[];
  windowStart: string | null;
  loading: boolean;
  error: string | null;
  lastUpdated: Date | null;
  /** Ids that arrived on the most recent poll, for the ticker highlight. */
  newIds: Set<string>;
  /** Force an immediate refresh (used by the manual refresh button). */
  refresh: () => void;
}

function sortKey(e: ContractEvent): string {
  // Newest first. ledger_closed_at is the primary ordering the API uses; id
  // breaks ties deterministically so the list does not reshuffle between polls.
  return `${e.ledger_closed_at}|${String(e.ledger).padStart(12, "0")}|${e.id}`;
}

export function useLiveFeed(
  refreshMs: number = REFRESH_MS,
  minutes: number = ACTIVITY_MINUTES
): LiveFeed {
  const [events, setEvents] = useState<ContractEvent[]>([]);
  const [contracts, setContracts] = useState<ContractEventRate[]>([]);
  const [windowStart, setWindowStart] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [newIds, setNewIds] = useState<Set<string>>(new Set());

  // Ids already rendered. A ref (not state) so merging does not depend on the
  // render cycle.
  const seen = useRef<Set<string>>(new Set());
  const isFirstLoad = useRef(true);
  const highlightTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const load = useCallback(async () => {
    try {
      const [recent, activity] = await Promise.all([
        getRecentEvents(TICKER_SIZE),
        getLiveActivity(minutes),
      ]);

      const incoming = recent.events ?? [];
      const fresh = incoming.filter((e) => !seen.current.has(e.id));
      for (const e of incoming) seen.current.add(e.id);

      if (fresh.length > 0 && !isFirstLoad.current) {
        setNewIds(new Set(fresh.map((e) => e.id)));
        if (highlightTimer.current) clearTimeout(highlightTimer.current);
        highlightTimer.current = setTimeout(
          () => setNewIds(new Set()),
          NEW_HIGHLIGHT_MS
        );
      }
      isFirstLoad.current = false;

      setEvents((prev) => {
        // Merge by id, newest wins, then re-sort and cap. Because the merge is
        // id-keyed, a poll with no new events produces an identical array and
        // React skips the re-render entirely.
        const byId = new Map<string, ContractEvent>();
        for (const e of incoming) byId.set(e.id, e);
        for (const e of prev) if (!byId.has(e.id)) byId.set(e.id, e);
        return [...byId.values()]
          .sort((a, b) => (sortKey(a) < sortKey(b) ? 1 : -1))
          .slice(0, TICKER_SIZE);
      });

      setContracts(activity.contracts ?? []);
      setWindowStart(activity.window_start ?? null);
      setLastUpdated(new Date());
      setError(null);
    } catch (err) {
      // Keep the last good data on screen; a wall display should degrade to
      // stale rather than blank.
      setError(err instanceof Error ? err.message : "Failed to load live data");
    } finally {
      setLoading(false);
    }
  }, [minutes]);

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setInterval> | null = null;

    const tick = () => {
      if (!cancelled) void load();
    };

    const start = () => {
      if (timer !== null) return;
      tick();
      timer = setInterval(tick, refreshMs);
    };
    const stop = () => {
      if (timer === null) return;
      clearInterval(timer);
      timer = null;
    };

    const onVisibility = () => {
      if (document.visibilityState === "visible") start();
      else stop();
    };

    if (document.visibilityState === "visible") start();
    document.addEventListener("visibilitychange", onVisibility);

    return () => {
      cancelled = true;
      stop();
      document.removeEventListener("visibilitychange", onVisibility);
      if (highlightTimer.current) clearTimeout(highlightTimer.current);
    };
  }, [load, refreshMs]);

  return {
    events,
    contracts,
    windowStart,
    loading,
    error,
    lastUpdated,
    newIds,
    refresh: () => void load(),
  };
}
