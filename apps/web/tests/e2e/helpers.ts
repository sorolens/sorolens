import { expect, type Page, type Route } from "@playwright/test";

// A well-formed Soroban contract id: exactly 56 characters, leading 'C',
// A-Z0-9 only. The tracking wizard validates this client-side (issue #140), so
// the fixture has to be the right length.
export const CONTRACT_ID =
  "CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5";

// Well-formed 64-hex transaction hashes for the cross-contract call-trace
// fixture (issue #264). The dashboard validates this shape client-side, so the
// fixture has to be the right length as well as valid hex.
export const TRACE_TX_HASH =
  "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90";

// A second valid transaction hash, used by the newer live event.
export const NEWER_TRACE_TX_HASH =
  "b1c2d3e4f5061728394a5b6c7d8e9f01b1c2d3e4f5061728394a5b6c7d8e9f01";

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
    // Must precede "contracts/" so the wizard's pre-flight check is not answered
    // by the contract-detail handler.
    "contracts/validate": (route) => {
      if (route.request().method() !== "POST") {
        return { status: 404, body: { error: "not mocked" } };
      }
      return {
        status: 200,
        body: {
          valid: true,
          contract_id: CONTRACT_ID,
          network: "testnet",
          already_tracked: false,
          label: null,
          reason: null,
        },
      };
    },
    // Live dashboard feeds (#139).
    "events/recent": () => ({
      status: 200,
      body: { events: [contractEvent()] },
    }),
    "stats/activity": () => ({
      status: 200,
      body: {
        minutes: 30,
        window_start: "2026-07-03T08:00:00Z",
        contracts: [contractEventRate()],
      },
    }),
    "contracts/": defaultContract,
    contracts: (route) => {
      if (route.request().method() === "POST") {
        return { status: 201, body: contractSummary() };
      }
      return {
        status: 200,
        body: { contracts: [contractSummary()], cursor: null, has_more: false },
      };
    },
    "stats/global": () => ({
      status: 200,
      body: {
        tracked_contracts: 12,
        total_events: 5041,
        total_invocations: 390,
        total_storage_entries: 128,
      },
    }),
    // Cross-contract call trace for a transaction (issue #264).
    "invocations/": () => ({ status: 200, body: traceResponse() }),
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
    contract_id: CONTRACT_ID,
    network: "testnet",
    ledger: 120_400,
    ledger_closed_at: "2026-07-03T08:00:00Z",
    tx_hash: TRACE_TX_HASH,
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

/** A second, newer event, for ticker-update assertions. */
export function newerContractEvent() {
  return {
    ...contractEvent(),
    id: "evt_2",
    ledger: 120_401,
    ledger_closed_at: "2026-07-03T08:00:05Z",
    tx_hash: NEWER_TRACE_TX_HASH,
  };
}

/**
 * A transaction's cross-contract call tree, as returned by
 * `GET /api/v1/invocations/{tx_hash}/trace` (issue #264). The root is the
 * transaction's own invocation; `transfer` is a cross-contract sub-invocation.
 */
export function traceResponse() {
  return {
    tx_hash: TRACE_TX_HASH,
    status: "SUCCESS",
    network: "testnet",
    ledger: 120_400,
    root: {
      span_id: "0",
      contract_id: CONTRACT_ID,
      function_name: "swap",
      cpu: 0,
      mem: 0,
      fee_share: 0,
      depth: 0,
      children: [
        {
          span_id: "0.0",
          parent_span_id: "0",
          contract_id: CONTRACT_ID,
          function_name: "transfer",
          cpu: 0,
          mem: 0,
          fee_share: 100,
          depth: 1,
          children: [],
        },
      ],
    },
    edge_count: 1,
    has_edges: true,
    truncated: false,
  };
}

/** One contract's live activity row, with a full 30-bucket series. */
export function contractEventRate() {
  return {
    contract_id: CONTRACT_ID,
    label: "Escrow DEX",
    network: "testnet",
    total: 6,
    per_minute: [...Array(29).fill(0), 6],
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
