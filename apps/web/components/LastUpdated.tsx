"use client";

import { useEffect, useState } from "react";
import { useLastUpdated } from "@/hooks/useLastUpdated";
import { formatClockTime, formatRelativeTime } from "@/lib/lastUpdated";

/**
 * Footer line telling users when the data on the current page was last
 * refreshed, so they can spot stale views.
 *
 * The timestamp is written by `fetchJson` on every successful API response, so
 * it advances on its own as pages refetch (navigation, network switch, the
 * events polling fallback). Rendering is hydration-safe: the server snapshot is
 * "never fetched", so the placeholder markup is reused on the client's first
 * render and the timestamp appears immediately after mount.
 */
export function LastUpdated() {
  const { timestamp, resource } = useLastUpdated();
  const [now, setNow] = useState(() => Date.now());

  // Keep the "x minutes ago" part honest while a page sits idle.
  useEffect(() => {
    if (timestamp === null) return;
    const id = setInterval(() => setNow(Date.now()), 30_000);
    return () => clearInterval(id);
  }, [timestamp]);

  if (timestamp === null) {
    return (
      <p
        data-testid="last-updated"
        data-state="pending"
        aria-live="polite"
        className="tabular-nums"
      >
        Last updated: no data yet
      </p>
    );
  }

  return (
    <p
      data-testid="last-updated"
      data-state="ready"
      aria-live="polite"
      className="tabular-nums"
    >
      Last updated{" "}
      <time dateTime={new Date(timestamp).toISOString()}>
        {formatClockTime(timestamp)}
      </time>{" "}
      <span className="text-[var(--color-text-secondary)]">
        ({formatRelativeTime(timestamp, now)})
      </span>
      {resource ? (
        <span className="text-[var(--color-text-secondary)]">
          {" "}
          · {resource}
        </span>
      ) : null}
    </p>
  );
}
