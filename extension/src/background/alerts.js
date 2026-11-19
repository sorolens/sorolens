// Background alert polling (#837).
//
// The service worker wakes on a chrome.alarms tick, reads the user's watchlist,
// fetches the raw alert feed, and turns alerts it has not seen before into
// notifications plus a toolbar badge. Everything pure lives here so the
// de-duplication and backoff rules can be unit tested under plain Node.

import { ALERTS, ALERT_SEVERITIES, LOCAL_KEYS } from "../shared/constants.js";
import { storageGet, storageSet } from "../shared/browser.js";

export const ALERT_POLL_ALARM = "sorolens:alert-poll";

/** @typedef {{ lastCheckedAt: number, seen: string[], unread: number, failures: number, backoffMinutes: number }} AlertState */

/** @returns {AlertState} */
export function emptyAlertState() {
  return {
    lastCheckedAt: 0,
    seen: [],
    unread: 0,
    failures: 0,
    backoffMinutes: 0,
  };
}

/** Coerces whatever is in storage into a usable state. */
export function normalizeAlertState(raw) {
  const state = emptyAlertState();
  if (!raw || typeof raw !== "object") return state;
  if (typeof raw.lastCheckedAt === "number" && raw.lastCheckedAt > 0) {
    state.lastCheckedAt = raw.lastCheckedAt;
  }
  if (Array.isArray(raw.seen)) {
    state.seen = raw.seen.filter((key) => typeof key === "string");
  }
  if (typeof raw.unread === "number" && raw.unread >= 0) {
    state.unread = Math.floor(raw.unread);
  }
  if (typeof raw.failures === "number" && raw.failures >= 0) {
    state.failures = Math.floor(raw.failures);
  }
  if (typeof raw.backoffMinutes === "number" && raw.backoffMinutes >= 0) {
    state.backoffMinutes = raw.backoffMinutes;
  }
  return state;
}

/** Stable identity for one alert, used for notification de-duplication. */
export function alertKey(alert) {
  if (!alert || typeof alert !== "object") return "";
  return [
    alert.contract_id || "",
    alert.tx_hash || "",
    alert.ledger ?? "",
    alert.severity || "",
    alert.rule || "",
    alert.timestamp || "",
  ].join("|");
}

/** @returns {number} epoch ms, or 0 when the timestamp is missing/invalid. */
export function alertTimestamp(alert) {
  const raw = alert && alert.timestamp;
  if (!raw) return 0;
  const parsed = Date.parse(raw);
  return Number.isNaN(parsed) ? 0 : parsed;
}

/** Alerts for contracts on the user's watchlist, newest first. */
export function filterWatchedAlerts(alerts, watchedContractIds) {
  const watched = new Set(watchedContractIds || []);
  return (alerts || []).filter(
    (alert) => alert && watched.has(alert.contract_id)
  );
}

/**
 * Alerts that are strictly newer than the last successful poll and were never
 * recorded before. `seen` is the de-duplication memory; `lastCheckedAt` alone
 * would miss an alert whose timestamp was delayed behind a later one.
 */
export function selectNewAlerts(alerts, { since = 0, seen = [] } = {}) {
  const known = new Set(seen || []);
  return (alerts || [])
    .filter((alert) => {
      const key = alertKey(alert);
      if (!key || known.has(key)) return false;
      if (!since) return false;
      return alertTimestamp(alert) > since;
    })
    .sort((a, b) => alertTimestamp(a) - alertTimestamp(b));
}

/** Whether this alert should raise a browser notification. */
export function shouldNotify(alert, settings) {
  if (!alert || (settings && settings.notificationsEnabled === false)) {
    return false;
  }
  const allowed = (settings && settings.notifySeverities) || [
    "Critical",
    "Warning",
  ];
  if (!Array.isArray(allowed)) return false;
  return (
    ALERT_SEVERITIES.includes(alert.severity) && allowed.includes(alert.severity)
  );
}

/**
 * Exponential backoff in minutes, honouring a Retry-After hint. The first
 * failure waits 2 minutes, then 4, 8, ... up to the configured cap.
 */
export function nextBackoffMinutes(failures, retryAfterSeconds = 0) {
  const exponent = Math.max(1, Number(failures) || 1);
  const base = Math.min(
    2 ** Math.min(exponent, 6),
    ALERTS.maxBackoffMinutes
  );
  const retryMinutes = Number(retryAfterSeconds) > 0
    ? Math.ceil(Number(retryAfterSeconds) / 60)
    : 0;
  return Math.min(
    Math.max(base, retryMinutes),
    ALERTS.maxBackoffMinutes
  );
}

/** Deterministic, identifier-safe notification id for one alert. */
export function notificationId(key) {
  const contract = String(key || "").split("|")[0] || "unknown";
  return `sorolens-alert-${contract}-${hashKey(key)}`;
}

/** Notification ids embed the contract so a click can open its page. */
export function notificationTarget(id) {
  const match = /^sorolens-alert-([A-Z0-9]+)-/.exec(id || "");
  return match ? match[1] : "";
}

function hashKey(key) {
  let hash = 0x811c9dc5;
  for (let i = 0; i < key.length; i += 1) {
    hash ^= key.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193) >>> 0;
  }
  return hash.toString(36);
}

/** @returns {Promise<AlertState>} */
export async function readAlertState() {
  const stored = await storageGet("local", { [LOCAL_KEYS.alertState]: null });
  return normalizeAlertState(stored[LOCAL_KEYS.alertState]);
}

/** @returns {Promise<AlertState>} the state that was written */
export async function writeAlertState(state) {
  const next = normalizeAlertState(state);
  await storageSet("local", { [LOCAL_KEYS.alertState]: next });
  return next;
}

/**
 * Runs one poll. The caller owns scheduling and retry: an API error is
 * propagated so it can apply `nextBackoffMinutes` with any `retryAfter` the
 * error carries.
 *
 * The first successful poll only establishes a baseline: historic alerts are
 * remembered but never notified.
 *
 * @param {object} options
 * @param {{ listWatchlist: () => Promise<string[]>, listAlertsFlat: (options?: object) => Promise<object[]> }} options.client
 * @param {object} options.settings
 * @param {AlertState} options.state
 * @param {number} [options.now]
 * @param {(alert: object) => Promise<void> | void} [options.notify]
 * @param {(unread: number) => Promise<void> | void} [options.setUnread]
 */
export async function pollOnce({
  client,
  settings,
  state,
  now = Date.now(),
  notify,
  setUnread,
}) {
  const current = normalizeAlertState(state);
  const watched = await client.listWatchlist();

  if (!watched || watched.length === 0) {
    const next = {
      ...current,
      lastCheckedAt: now,
      seen: [],
      unread: 0,
      failures: 0,
      backoffMinutes: 0,
    };
    await writeAlertState(next);
    if (setUnread) await setUnread(0);
    return { state: next, newAlerts: [] };
  }

  const alerts = await client.listAlertsFlat({ limit: ALERTS.fetchLimit });
  const candidates = filterWatchedAlerts(alerts, watched);
  const baseline = current.lastCheckedAt === 0;
  const newAlerts = baseline
    ? []
    : selectNewAlerts(candidates, {
        since: current.lastCheckedAt,
        seen: current.seen,
      });

  // Remember every candidate on the baseline poll and every genuinely new
  // alert afterwards. Capped so sync/local storage stays small.
  const remembered = baseline
    ? candidates.map((alert) => alertKey(alert)).filter(Boolean)
    : newAlerts.map((alert) => alertKey(alert)).filter(Boolean);
  const seen = [...current.seen, ...remembered].slice(-ALERTS.seenLimit);

  const unread = baseline ? 0 : current.unread + newAlerts.length;
  const next = {
    ...current,
    lastCheckedAt: now,
    seen,
    unread,
    failures: 0,
    backoffMinutes: 0,
  };
  await writeAlertState(next);

  for (const alert of newAlerts) {
    if (shouldNotify(alert, settings) && notify) {
      await notify(alert);
    }
  }
  if (setUnread) await setUnread(unread);

  return { state: next, newAlerts };
}
