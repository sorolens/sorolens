import { test, expect, type Route } from "@playwright/test";
import { expectHeading, mockApi } from "./helpers";

type ApiHandler = (route: Route) => { status: number; body: unknown };

/**
 * E2E coverage for the contract comparison view (#766): selecting and removing
 * contracts, the four-contract cap, URL-synced shareable links and the error
 * state.
 */

function contractId(letter: string): string {
  return `C${letter.repeat(55)}`;
}

const CONTRACTS = [
  { id: contractId("A"), label: "Alpha" },
  { id: contractId("B"), label: "Beta" },
  { id: contractId("D"), label: "Delta" },
  { id: contractId("E"), label: "Epsilon" },
  { id: contractId("F"), label: "Foxtrot" },
];

const byId = new Map(CONTRACTS.map((contract) => [contract.id, contract]));

function contractSummary(contract: { id: string; label: string }) {
  return {
    id: contract.id,
    network: "testnet",
    label: contract.label,
    wasm_hash: "3c1b2d9f",
    created_at_ledger: 119_000,
    backfill_complete_at: null,
    status: "active",
    added_at: "2026-11-01T00:00:00Z",
    tags: [],
  };
}

function compareEntry(id: string) {
  return {
    id,
    network: "testnet",
    label: byId.get(id)?.label ?? "",
    status: "active",
    tracked: true,
    has_data: true,
    event_count: 10,
    invocation_count: 4,
    avg_cpu: 1200.5,
    avg_fee: 30,
    health_score: 92,
    health_components: {
      uptime: 95,
      error_rate: 90,
      performance: 88,
      storage_ttl: 100,
    },
    last_synced_ledger: 120_400,
    event_volume: [
      { timestamp: "2026-11-01T00:00:00Z", count: 2 },
      { timestamp: "2026-11-01T00:05:00Z", count: 3 },
    ],
  };
}

const CONTRACTS_RESPONSE = {
  contracts: CONTRACTS.map(contractSummary),
  next_cursor: "",
};

const COMPARE_RESPONSE = {
  window: "7d",
  contracts: [compareEntry(CONTRACTS[0].id), compareEntry(CONTRACTS[1].id)],
};

const contractsHandler: ApiHandler = () => ({
  status: 200,
  body: CONTRACTS_RESPONSE,
});

function compareHandler(requests: URLSearchParams[], fail = false): ApiHandler {
  return (route) => {
    const params = new URL(route.request().url()).searchParams;
    requests.push(params);
    if (fail) {
      return { status: 500, body: { error: "comparison failed" } };
    }
    const ids = (params.get("ids") ?? "").split(",").filter(Boolean);
    return {
      status: 200,
      body: {
        window: params.get("window") ?? "7d",
        contracts: ids.map(compareEntry),
      },
    };
  };
}

test("/compare renders a shared comparison link", async ({ page }) => {
  const requests: URLSearchParams[] = [];
  await mockApi(page, {
    "api/v1/contracts": contractsHandler,
    "api/v1/compare": compareHandler(requests),
  });
  await page.goto(`/compare?ids=${CONTRACTS[0].id},${CONTRACTS[1].id}`);

  await expectHeading(page, "Compare Contracts");
  for (const contract of COMPARE_RESPONSE.contracts) {
    await expect(
      page.getByRole("heading", { name: contract.label })
    ).toBeVisible();
  }
  await expect(page.getByText("2/4 selected")).toBeVisible();
  await expect(page.getByText("Events", { exact: true }).first()).toBeVisible();

  expect(requests[0].get("ids")).toBe(`${CONTRACTS[0].id},${CONTRACTS[1].id}`);
  expect(requests[0].get("window")).toBe("7d");
});

test("/compare adds and removes contracts and mirrors the selection in the URL", async ({
  page,
}) => {
  const requests: URLSearchParams[] = [];
  await mockApi(page, {
    "api/v1/contracts": contractsHandler,
    "api/v1/compare": compareHandler(requests),
  });
  await page.goto("/compare");

  await expect(
    page.getByText("Select at least 2 contracts to compare")
  ).toBeVisible();

  await page.getByRole("button", { name: "Alpha", exact: true }).click();
  await expect(page.getByText("1/4 selected")).toBeVisible();
  await expect(
    page.getByText("Select at least 2 contracts", { exact: true })
  ).toBeVisible();

  await page.getByRole("button", { name: "Beta", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Alpha" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Beta" })).toBeVisible();
  await expect(page.getByText("Ready to compare")).toBeVisible();

  // The selection is encoded in the URL after the client-side router.replace.
  await expect(page).toHaveURL(
    new RegExp(`ids=${CONTRACTS[0].id},${CONTRACTS[1].id}`)
  );
  expect(requests[requests.length - 1].get("ids")).toBe(
    `${CONTRACTS[0].id},${CONTRACTS[1].id}`
  );

  // Removing a contract drops its column and returns to the prompt state.
  await page.getByRole("button", { name: `Remove ${CONTRACTS[1].id}` }).click();
  await expect(page.getByRole("heading", { name: "Beta" })).toBeHidden();
  await expect(page.getByText("1/4 selected")).toBeVisible();
  await expect(
    page.getByText("Select at least 2 contracts to compare")
  ).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`ids=${CONTRACTS[0].id}`));
});

test("/compare caps the selection at four contracts", async ({ page }) => {
  await mockApi(page, {
    "api/v1/contracts": contractsHandler,
    "api/v1/compare": compareHandler([]),
  });
  await page.goto("/compare");

  for (const contract of CONTRACTS.slice(0, 4)) {
    await page
      .getByRole("button", { name: contract.label, exact: true })
      .click();
  }

  await expect(page.getByText("4/4 selected")).toBeVisible();
  await expect(
    page.getByRole("button", { name: CONTRACTS[4].label, exact: true })
  ).toBeDisabled();
  await expect(page.locator('[data-testid="compare-slot-filled"]')).toHaveCount(
    4
  );
});

test("/compare drops extra ids from a shared link and explains why", async ({
  page,
}) => {
  await mockApi(page, {
    "api/v1/contracts": contractsHandler,
    "api/v1/compare": compareHandler([]),
  });
  const ids = CONTRACTS.map((contract) => contract.id).join(",");
  await page.goto(`/compare?ids=${ids}`);

  const note = page.getByTestId("compare-url-note");
  await expect(note).toBeVisible();
  await expect(note).toContainText(
    "A comparison holds 4 contracts, so 1 more was dropped."
  );
  await expect(page.getByText("4/4 selected")).toBeVisible();
});

test("/compare ignores malformed ids from a shared link", async ({ page }) => {
  await mockApi(page, {
    "api/v1/contracts": contractsHandler,
    "api/v1/compare": compareHandler([]),
  });
  await page.goto(
    `/compare?ids=${CONTRACTS[0].id},not-a-contract-id,${CONTRACTS[1].id}`
  );

  const note = page.getByTestId("compare-url-note");
  await expect(note).toBeVisible();
  await expect(note).toContainText("1 invalid contract ID was ignored");
  await expect(page.getByRole("heading", { name: "Alpha" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Beta" })).toBeVisible();
});

test("/compare keeps a selected comparison shareable across loads", async ({
  page,
  context,
}) => {
  await mockApi(page, {
    "api/v1/contracts": contractsHandler,
    "api/v1/compare": compareHandler([]),
  });
  await page.goto("/compare");
  await page.getByRole("button", { name: "Alpha", exact: true }).click();
  await page.getByRole("button", { name: "Beta", exact: true }).click();
  await expect(page).toHaveURL(/ids=/);

  const sharedUrl = page.url();

  // A fresh tab opening the copied URL sees the same two contracts.
  const shared = await context.newPage();
  await mockApi(shared, {
    "api/v1/contracts": contractsHandler,
    "api/v1/compare": compareHandler([]),
  });
  await shared.goto(sharedUrl);
  await expect(shared.getByRole("heading", { name: "Alpha" })).toBeVisible();
  await expect(shared.getByRole("heading", { name: "Beta" })).toBeVisible();
  await expect(shared.getByText("2/4 selected")).toBeVisible();
  await shared.close();
});

test("/compare shows the API error instead of a partial comparison", async ({
  page,
}) => {
  await mockApi(page, {
    "api/v1/contracts": contractsHandler,
    "api/v1/compare": compareHandler([], true),
  });
  await page.goto(`/compare?ids=${CONTRACTS[0].id},${CONTRACTS[1].id}`);

  // Next's route announcer is also a role=alert, so match the message text.
  await expect(page.getByText("comparison failed")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Alpha" })).toBeHidden();
});
