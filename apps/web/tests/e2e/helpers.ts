import { expect, type Page, type Route } from "@playwright/test";

export const CONTRACT_ID =
  "CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C33";

type Handler = (
  route: Route,
  page: Page
) => {
  status: number;
  body: unknown;
};

/** Overridable per-endpoint handlers, matched by pathname substring. */
export function defaultHandlers(): Record<string, Handler> {
  return {
    "watchdog/contracts/": defaultContract,
    "watchdog/contracts": () => ({
      status: 200,
      body: { contracts: [monitoredContract()], next_cursor: "" },
    }),
    "watchdog/alerts": () => ({ status: 200, body: { alerts: [] } }),
    "watchdog/stats": () => ({ status: 200, body: watchdogStats() }),
    "contracts/": defaultContract,
    contracts: () => ({
      status: 200,
      body: { contracts: [contractSummary()], cursor: null, has_more: false },
    }),
    "stats/global": () => ({
      status: 200,
      body: {
        tracked_contracts: 12,
        total_events: 5041,
        total_invocations: 390,
        total_storage_entries: 128,
      },
    }),
  };
}

function defaultContract(route: Route): { status: number; body: unknown } {
  const path = new URL(route.request().url()).pathname;
  const qs = new URL(route.request().url()).searchParams;
  if (path.endsWith("/events")) {
    return {
      status: 200,
      body: { events: [contractEvent()], cursor: null, has_more: false },
    };
  }
  if (path.endsWith("/storage")) {
    return {
      status: 200,
      body: {
        current_ledger: 42,
        entries: [storageEntry()],
        cursor: null,
        has_more: false,
      },
    };
  }
  if (path.endsWith("/stats")) {
    return { status: 200, body: statsResponse() };
  }
  if (path.endsWith("/health-score")) {
    return { status: 404, body: { error: "not found" } };
  }
  if (path.endsWith("/snapshot")) {
    return { status: 404, body: { error: "not found" } };
  }
  if (path.endsWith("/uptime")) {
    const window = qs.get("window") ?? "24h";
    return {
      status: 200,
      body: { contract_id: CONTRACT_ID, window, uptime_pct: 99.85 },
    };
  }
  if (path.endsWith("/health")) {
    return { status: 200, body: { health_checks: [] } };
  }
  if (path.endsWith("/alerts")) {
    return { status: 200, body: { alerts: [] } };
  }
  return { status: 200, body: contractDetail() };
}

/** Installs mocked API responses via route interception. */
export async function mockApi(
  page: Page,
  overrides: Record<string, Handler> = {}
): Promise<void> {
  const handlers = { ...defaultHandlers(), ...overrides };
  await page.route("**/api/v1/**", (route) => {
    const path = new URL(route.request().url()).pathname;
    const key = Object.keys(handlers).find((k) => path.includes(k));
    if (!key) {
      return route.fulfill({
        status: 404,
        contentType: "application/json",
        body: JSON.stringify({ error: "not mocked" }),
      });
    }
    const { status, body } = handlers[key](route, page);
    return route.fulfill({
      status,
      contentType: "application/json",
      body: JSON.stringify(body),
    });
  });
}

/** Aborts every API request so the dashboard shows its unreachable state. */
export async function blockApi(page: Page): Promise<void> {
  await page.route("**/api/v1/**", (route) => route.abort("connectionrefused"));
}

const emptyWatchdogContracts: Handler = () => ({
  status: 200,
  body: { contracts: [], next_cursor: "" },
});
const emptyWatchdogAlerts: Handler = () => ({
  status: 200,
  body: { alerts: [] },
});

/** Watchdog endpoints returning empty lists (no data yet). */
export function emptyWatchdogHandlers(): Record<string, Handler> {
  return {
    "watchdog/contracts": emptyWatchdogContracts,
    "watchdog/alerts": emptyWatchdogAlerts,
  };
}

/**
 * Watchdog contract-detail endpoints. The contract itself, its health-check
 * history, and its alerts all live under `/watchdog/contracts/{id}`, so one
 * handler branches on the path suffix.
 */
export function watchdogDetailHandlers(
  healthChecks: unknown[]
): Record<string, Handler> {
  return {
    "watchdog/contracts/": (route) => {
      const url = new URL(route.request().url());
      const path = url.pathname;
      if (path.endsWith("/health")) {
        return { status: 200, body: { health_checks: healthChecks } };
      }
      if (path.endsWith("/alerts")) {
        return { status: 200, body: { alerts: [] } };
      }
      if (path.endsWith("/uptime")) {
        const window = url.searchParams.get("window") ?? "24h";
        return {
          status: 200,
          body: { contract_id: CONTRACT_ID, window, uptime_pct: 99.85 },
        };
      }
      return { status: 200, body: monitoredContract() };
    },
  };
}

export function contractSummary() {
  return {
    id: CONTRACT_ID,
    network: "testnet",
    label: "Escrow DEX",
    status: "active",
    wasm_hash: "3c1b2d9f",
    added_at: "2026-07-02T10:15:00Z",
    last_activity_at: "2026-07-02T10:14:00Z",
  };
}

export function contractDetail() {
  return {
    ...contractSummary(),
    backfill_complete_at: null,
    sync: { last_ledger: 120_400, last_run_at: "2026-07-03T08:00:00Z" },
    storage_entry_count: 128,
    expiring_entry_count: 3,
  };
}

export function monitoredContract() {
  return {
    contract_id: CONTRACT_ID,
    network: "testnet",
    name: "Escrow DEX",
    owner: "GCQH32AZF6PXY2R4T3M5Q7XFYVK2TLQD2TELC7K4BXL5Q6T7P9U2KNQT",
    status: "Healthy",
    last_check: "2026-07-03T08:00:00Z",
    check_interval: 60,
    registered_at: "2026-06-28T12:00:00Z",
    updated_at: "2026-07-03T08:00:00Z",
  };
}

export function contractEvent() {
  return {
    id: "evt_1",
    ledger: 120_400,
    ledger_closed_at: "2026-07-03T08:00:00Z",
    tx_hash: "a1b2c3",
    type: "transfer",
    topic_decoded: [
      "transfer",
      "GD2E2SJSD2C5SQGMH2YPIXI6F7NS2S6MZ2G4T3VDWXQH2FQT4M6URPLT",
    ],
    topic_xdr: ["AAAAAA=="],
    value_decoded: { amount: 1000 },
    value_xdr: "AAAAAw==",
    in_successful_call: true,
  };
}

export function storageEntry() {
  return {
    key_xdr: "AAAAAQ==",
    key_decoded: "balance",
    value_xdr: "AAAAAQ==",
    value_decoded: "1000",
    durability: "persistent",
    live_until_ledger: null,
    ledgers_until_expiry: null,
    status: "live",
    last_modified_ledger: 120_400,
  };
}

export function statsResponse() {
  return {
    event_volume: [{ date: "2026-07-03", ledger: 120_400, count: 12 }],
    invocation_count: [{ date: "2026-07-03", ledger: 120_400, count: 4 }],
    stats: {
      total_events: 5041,
      total_invocations: 390,
      storage_entry_count: 128,
      expiring_entry_count: 3,
    },
  };
}

export function watchdogStats() {
  return {
    total_monitored: 1,
    healthy: 1,
    degraded: 0,
    unresponsive: 0,
    total_alerts: 0,
    critical_alerts: 0,
  };
}

export async function expectHeading(
  page: Page,
  name: string | RegExp,
  options: { exact?: boolean } = {}
) {
  await expect(
    page.getByRole("heading", { name, exact: options.exact ?? true })
  ).toBeVisible();
}
