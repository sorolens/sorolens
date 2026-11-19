import test, { after } from "node:test";
import assert from "node:assert/strict";

import {
  alertKey,
  filterWatchedAlerts,
  nextBackoffMinutes,
  notificationId,
  notificationTarget,
  normalizeAlertState,
  pollOnce,
  selectNewAlerts,
  shouldNotify,
} from "../src/background/alerts.js";
import {
  CONTRACT_ID,
  OTHER_CONTRACT_ID,
  installFakeBrowser,
} from "./fixtures.mjs";

// pollOnce persists the alert state, so the storage shim must be present for
// the whole file. Pure helpers do not touch it.
const browser = installFakeBrowser();
after(() => browser.restore());

const NOW = Date.parse("2026-11-19T12:00:00Z");

function alert(overrides = {}) {
  return {
    contract_id: CONTRACT_ID,
    severity: "Critical",
    message: "Downtime detected",
    ledger: 100,
    tx_hash: "a".repeat(64),
    timestamp: "2026-11-19T11:59:00Z",
    ...overrides,
  };
}

function fakeClient({ watched = [CONTRACT_ID], alerts = [alert()] } = {}) {
  return {
    listWatchlist: async () => watched,
    listAlertsFlat: async () => alerts,
  };
}

function settings(overrides = {}) {
  return {
    notificationsEnabled: true,
    notifySeverities: ["Critical", "Warning"],
    ...overrides,
  };
}

test("normalizeAlertState coerces missing and malformed fields", () => {
  assert.deepEqual(normalizeAlertState(null), {
    lastCheckedAt: 0,
    seen: [],
    unread: 0,
    failures: 0,
    backoffMinutes: 0,
  });
  assert.deepEqual(
    normalizeAlertState({
      lastCheckedAt: 10,
      seen: ["a", 3, "b"],
      unread: 2.7,
      failures: -1,
    }),
    {
      lastCheckedAt: 10,
      seen: ["a", "b"],
      unread: 2,
      failures: 0,
      backoffMinutes: 0,
    }
  );
});

test("filterWatchedAlerts keeps only watchlisted contracts", () => {
  const alerts = [
    alert(),
    alert({ contract_id: OTHER_CONTRACT_ID }),
    alert({ contract_id: undefined }),
  ];
  assert.deepEqual(filterWatchedAlerts(alerts, [CONTRACT_ID]), [alerts[0]]);
});

test("selectNewAlerts ignores old and already-seen alerts", () => {
  const seenAlert = alert({ tx_hash: "b".repeat(64) });
  const fresh = alert({
    tx_hash: "c".repeat(64),
    timestamp: "2026-11-19T11:59:30Z",
  });
  const old = alert({
    tx_hash: "d".repeat(64),
    timestamp: "2026-11-19T11:00:00Z",
  });

  const result = selectNewAlerts([seenAlert, fresh, old], {
    since: NOW - 10 * 60_000,
    seen: [alertKey(seenAlert)],
  });
  assert.deepEqual(result, [fresh]);
});

test("alertKey changes with the alert identity", () => {
  const a = alert();
  assert.notEqual(alertKey(a), alertKey({ ...a, tx_hash: "z".repeat(64) }));
  assert.equal(alertKey(a), alertKey({ ...a }));
});

test("shouldNotify respects the enabled flag and severity filter", () => {
  assert.equal(shouldNotify(alert(), settings()), true);
  assert.equal(shouldNotify(alert({ severity: "Info" }), settings()), false);
  assert.equal(
    shouldNotify(alert(), settings({ notificationsEnabled: false })),
    false
  );
  assert.equal(
    shouldNotify(alert(), settings({ notifySeverities: ["Info"] })),
    false
  );
});

test("nextBackoffMinutes grows exponentially and honours Retry-After", () => {
  assert.equal(nextBackoffMinutes(1), 2);
  assert.equal(nextBackoffMinutes(2), 4);
  assert.equal(nextBackoffMinutes(3), 8);
  assert.equal(nextBackoffMinutes(10), 60);
  assert.equal(nextBackoffMinutes(1, 600), 10);
  assert.equal(nextBackoffMinutes(1, 86_400), 60);
});

test("notification ids are deterministic and yield the contract id", () => {
  const id = notificationId(alertKey(alert()));
  assert.equal(id, notificationId(alertKey(alert())));
  assert.equal(notificationTarget(id), CONTRACT_ID);
  assert.equal(notificationTarget("something-else"), "");
});

test("first poll establishes a baseline without notifying", async () => {
  const notifications = [];
  const badges = [];
  const result = await pollOnce({
    client: fakeClient(),
    settings: settings(),
    state: normalizeAlertState(null),
    now: NOW,
    notify: (a) => notifications.push(a),
    setUnread: (n) => badges.push(n),
  });

  assert.equal(notifications.length, 0);
  assert.equal(result.newAlerts.length, 0);
  assert.equal(result.state.lastCheckedAt, NOW);
  assert.equal(result.state.seen.length, 1);
  assert.deepEqual(badges, [0]);
});

test("a new alert notifies once, updates the badge and is de-duplicated", async () => {
  const notifications = [];
  const badges = [];
  const state = {
    lastCheckedAt: NOW - 10 * 60_000,
    seen: [],
    unread: 0,
    failures: 0,
    backoffMinutes: 0,
  };
  const first = await pollOnce({
    client: fakeClient(),
    settings: settings(),
    state,
    now: NOW,
    notify: (a) => notifications.push(alertKey(a)),
    setUnread: (n) => badges.push(n),
  });

  assert.equal(notifications.length, 1);
  assert.equal(first.state.unread, 1);
  assert.equal(badges.at(-1), 1);

  // The same feed is fetched again: the alert is remembered, so no second
  // notification and no extra unread count.
  const second = await pollOnce({
    client: fakeClient(),
    settings: settings(),
    state: first.state,
    now: NOW + 5 * 60_000,
    notify: (a) => notifications.push(alertKey(a)),
    setUnread: (n) => badges.push(n),
  });

  assert.equal(notifications.length, 1);
  assert.equal(second.newAlerts.length, 0);
  assert.equal(second.state.unread, 1);
});

test("severity-filtered alerts still count towards unread", async () => {
  const notifications = [];
  const result = await pollOnce({
    client: fakeClient({
      alerts: [alert({ severity: "Info", tx_hash: "e".repeat(64) })],
    }),
    settings: settings({ notifySeverities: ["Critical"] }),
    state: {
      lastCheckedAt: NOW - 2 * 60_000,
      seen: [],
      unread: 0,
      failures: 0,
      backoffMinutes: 0,
    },
    now: NOW,
    notify: (a) => notifications.push(a),
    setUnread: () => undefined,
  });

  assert.equal(notifications.length, 0);
  assert.equal(result.newAlerts.length, 1);
  assert.equal(result.state.unread, 1);
});

test("an empty watchlist clears the badge and the seen memory", async () => {
  const badges = [];
  const result = await pollOnce({
    client: fakeClient({ watched: [] }),
    settings: settings(),
    state: {
      lastCheckedAt: 1,
      seen: ["old"],
      unread: 7,
      failures: 2,
      backoffMinutes: 8,
    },
    now: NOW,
    setUnread: (n) => badges.push(n),
  });

  assert.deepEqual(badges, [0]);
  assert.equal(result.state.unread, 0);
  assert.deepEqual(result.state.seen, []);
  assert.equal(result.state.failures, 0);
});
