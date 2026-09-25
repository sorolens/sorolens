/**
 * Tracks when the dashboard's data was last refreshed.
 *
 * Every dashboard page fetches through `lib/api.ts`, so `fetchJson` records a
 * timestamp here after each successful response and the footer subscribes to
 * it. Keeping the store outside React means the timestamp survives client-side
 * navigation between pages instead of resetting on every unmount.
 *
 * `getServerLastUpdated` always reports "never fetched", so the server-rendered
 * markup and the first client render are identical and hydration stays
 * deterministic; the real timestamp appears on the render after mount.
 */

export interface LastUpdatedSnapshot {
  /** Epoch milliseconds of the last successful fetch, or `null`. */
  timestamp: number | null;
  /** Short label for the resource that was refreshed (e.g. "watchdog"). */
  resource: string | null;
}

const NEVER_UPDATED: LastUpdatedSnapshot = { timestamp: null, resource: null };

let snapshot: LastUpdatedSnapshot = NEVER_UPDATED;
const listeners = new Set<() => void>();

export function getLastUpdated(): LastUpdatedSnapshot {
  return snapshot;
}

/** Server/hydration snapshot: the footer shows its placeholder until mount. */
export function getServerLastUpdated(): LastUpdatedSnapshot {
  return NEVER_UPDATED;
}

export function subscribeLastUpdated(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/**
 * Record a successful fetch. Subscribers are only notified when the snapshot
 * actually changes, so repeated identical reads stay render-free.
 */
export function recordLastUpdated(
  resource: string,
  at: number = Date.now(),
): void {
  if (snapshot.timestamp === at && snapshot.resource === resource) return;
  snapshot = { timestamp: at, resource };
  for (const listener of listeners) listener();
}

/** Clear the store. Used by tests and when the active network changes. */
export function resetLastUpdated(): void {
  if (snapshot === NEVER_UPDATED) return;
  snapshot = NEVER_UPDATED;
  for (const listener of listeners) listener();
}

const SECOND = 1_000;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/**
 * Render the elapsed time since `timestamp` as a short label ("just now",
 * "3m ago", "2h ago", "4d ago"). `now` is injectable so tests do not depend on
 * the wall clock.
 */
export function formatRelativeTime(
  timestamp: number,
  now: number = Date.now(),
): string {
  if (!Number.isFinite(timestamp)) return "";
  const elapsed = Math.max(0, now - timestamp);
  if (elapsed < MINUTE) return "just now";
  if (elapsed < HOUR) return `${Math.floor(elapsed / MINUTE)}m ago`;
  if (elapsed < DAY) return `${Math.floor(elapsed / HOUR)}h ago`;
  return `${Math.floor(elapsed / DAY)}d ago`;
}

/** Absolute, locale-aware clock label, e.g. "14:32:07". */
export function formatClockTime(timestamp: number, locales?: string): string {
  if (!Number.isFinite(timestamp)) return "";
  return new Date(timestamp).toLocaleTimeString(locales, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

const RESOURCE_LABELS: Array<[RegExp, string]> = [
  [/\/api\/v1\/watchdog/, "watchdog"],
  [/\/api\/v1\/watchlist/, "watchlist"],
  [/\/api\/v1\/stats\/global/, "global stats"],
  [/\/api\/v1\/contracts\/[^/?]+\/events/, "events"],
  [/\/api\/v1\/contracts\/[^/?]+\/invocations/, "invocations"],
  [/\/api\/v1\/contracts\/[^/?]+\/storage/, "storage"],
  [/\/api\/v1\/contracts\/[^/?]+\/stats/, "stats"],
  [/\/api\/v1\/contracts\/[^/?]+\/snapshot/, "snapshot"],
  [/\/api\/v1\/contracts\/[^/?]+\/health-score/, "health"],
  [/\/api\/v1\/contracts/, "contracts"],
];

/**
 * Derive a short resource label from an API URL so the footer can say which
 * view was refreshed. Unknown paths fall back to "dashboard".
 */
export function resourceFromUrl(url: string): string {
  const path = url.split("?")[0];
  for (const [pattern, label] of RESOURCE_LABELS) {
    if (pattern.test(path)) return label;
  }
  return "dashboard";
}
