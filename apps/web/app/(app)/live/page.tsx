"use client";

/**
 * /live — real-time dashboard (issue #139).
 *
 * A full-height, self-refreshing view meant to be left up on a monitor in an
 * ops room: a ticker of the newest events, per-contract sparklines of events
 * per minute, and a hot-contracts leaderboard. It polls every 5 seconds and
 * updates in place; nothing on this page triggers a navigation or a reload.
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { EventTicker } from "@/components/live/EventTicker";
import { HotContracts } from "@/components/live/HotContracts";
import {
  ACTIVITY_MINUTES,
  REFRESH_MS,
  useLiveFeed,
} from "@/components/live/useLiveFeed";

/**
 * Fullscreen API surface.
 *
 * Safari only shipped the standard Fullscreen API recently and for a long time
 * required the `webkit`-prefixed variants, so both are probed. The acceptance
 * criterion is "works in fullscreen on Chrome and Safari", hence the fallbacks
 * rather than a plain `requestFullscreen()`.
 */
interface WebkitFullscreenElement extends HTMLElement {
  webkitRequestFullscreen?: () => Promise<void> | void;
}

interface WebkitFullscreenDocument extends Document {
  webkitFullscreenElement?: Element | null;
  webkitExitFullscreen?: () => Promise<void> | void;
}

function currentFullscreenElement(): Element | null {
  const doc = document as WebkitFullscreenDocument;
  return doc.fullscreenElement ?? doc.webkitFullscreenElement ?? null;
}

function formatClock(d: Date | null): string {
  if (!d) return "—";
  return d.toLocaleTimeString(undefined, { hour12: false });
}

export default function LivePage() {
  const containerRef = useRef<HTMLDivElement>(null);
  const { events, contracts, loading, error, lastUpdated, newIds, refresh } =
    useLiveFeed(REFRESH_MS, ACTIVITY_MINUTES);

  const [isFullscreen, setIsFullscreen] = useState(false);
  const [fullscreenError, setFullscreenError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  // Track fullscreen state from the event rather than from our own toggle, so
  // leaving fullscreen with Esc or F11 keeps the button label correct.
  useEffect(() => {
    const doc = document as WebkitFullscreenDocument;
    const onChange = () => setIsFullscreen(Boolean(currentFullscreenElement()));

    doc.addEventListener("fullscreenchange", onChange);
    doc.addEventListener("webkitfullscreenchange", onChange);
    onChange();

    return () => {
      doc.removeEventListener("fullscreenchange", onChange);
      doc.removeEventListener("webkitfullscreenchange", onChange);
    };
  }, []);

  const toggleFullscreen = useCallback(async () => {
    const doc = document as WebkitFullscreenDocument;
    const target = (containerRef.current ?? document.documentElement) as WebkitFullscreenElement;
    setPending(true);
    setFullscreenError(null);
    try {
      if (currentFullscreenElement()) {
        if (doc.exitFullscreen) await doc.exitFullscreen();
        else if (doc.webkitExitFullscreen) await doc.webkitExitFullscreen();
      } else if (target.requestFullscreen) {
        await target.requestFullscreen();
      } else if (target.webkitRequestFullscreen) {
        await target.webkitRequestFullscreen();
      } else {
        throw new Error("This browser does not support fullscreen");
      }
    } catch (err) {
      setFullscreenError(
        err instanceof Error ? err.message : "Could not enter fullscreen",
      );
    } finally {
      setPending(false);
    }
  }, []);

  return (
    <div
      ref={containerRef}
      data-testid="live-dashboard"
      className={[
        "flex flex-col gap-4",
        isFullscreen
          ? "h-screen bg-[var(--color-bg-page)] p-4"
          : "h-[calc(100vh-7rem)] min-h-[32rem]",
      ].join(" ")}
    >
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">
            Live
          </h1>
          <p className="text-xs text-[var(--color-text-secondary)]">
            Refreshes every {REFRESH_MS / 1000}s · last update{" "}
            <span data-testid="last-updated">{formatClock(lastUpdated)}</span>
          </p>
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            data-testid="live-refresh"
            onClick={refresh}
            className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] hover:border-[var(--color-accent)]"
          >
            Refresh
          </button>
          <button
            type="button"
            data-testid="fullscreen-toggle"
            onClick={toggleFullscreen}
            disabled={pending}
            aria-pressed={isFullscreen}
            className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] disabled:opacity-50"
          >
            {isFullscreen ? "Exit fullscreen" : "Fullscreen"}
          </button>
        </div>
      </div>

      {(error || fullscreenError) && (
        <p
          role="alert"
          data-testid="live-error"
          className="rounded-md border border-[var(--color-danger)]/40 bg-[var(--color-danger)]/10 px-3 py-2 text-sm text-[var(--color-danger)]"
        >
          {fullscreenError ?? error}
        </p>
      )}

      {/* Panels */}
      <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-5">
        <section className="flex min-h-0 flex-col overflow-hidden rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] lg:col-span-3">
          <h2 className="shrink-0 border-b border-[var(--color-border)] px-4 py-2.5 text-sm font-semibold uppercase tracking-wide text-[var(--color-text-secondary)]">
            Event ticker
          </h2>
          <div className="min-h-0 flex-1 overflow-y-auto">
            {loading && events.length === 0 ? (
              <p
                data-testid="ticker-loading"
                className="px-4 py-8 text-center text-sm text-[var(--color-text-secondary)]"
              >
                Loading events…
              </p>
            ) : (
              <EventTicker events={events} newIds={newIds} />
            )}
          </div>
        </section>

        <section className="flex min-h-0 flex-col overflow-hidden rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] lg:col-span-2">
          <h2 className="shrink-0 border-b border-[var(--color-border)] px-4 py-2.5 text-sm font-semibold uppercase tracking-wide text-[var(--color-text-secondary)]">
            Hot contracts · last {ACTIVITY_MINUTES} min
          </h2>
          <div className="min-h-0 flex-1 overflow-y-auto">
            <HotContracts contracts={contracts} minutes={ACTIVITY_MINUTES} />
          </div>
        </section>
      </div>
    </div>
  );
}
