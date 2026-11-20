import { test, expect, type Route } from "@playwright/test";
import { blockApi, CONTRACT_ID, expectHeading, mockApi } from "./helpers";

type ApiHandler = (route: Route) => { status: number; body: unknown };

/**
 * E2E coverage for the cross-contract invocations explorer (#766): filters,
 * pagination, empty and error states, and the URL-synced deep links added in
 * #618.
 */

const SECOND_CONTRACT_ID =
  "CBVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5";

function invocation(overrides: Record<string, unknown> = {}) {
  return {
    tx_hash: "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90",
    contract_id: CONTRACT_ID,
    network: "testnet",
    ledger: 120_400,
    ledger_closed_at: "2026-11-01T08:00:00Z",
    status: "SUCCESS",
    function_name: "swap",
    args_decoded: {},
    result_decoded: null,
    result_xdr: "AAAA",
    resource_fee_charged: 1200,
    cpu_insn: 140_000,
    mem_byte: 64_000,
    ledger_read_byte: 2048,
    ledger_write_byte: 512,
    application_order: 1,
    ...overrides,
  };
}

const FIRST_PAGE = {
  invocations: [
    invocation(),
    invocation({
      tx_hash:
        "b1c2d3e4f5061728394a5b6c7d8e9f01b1c2d3e4f5061728394a5b6c7d8e9f01",
      contract_id: SECOND_CONTRACT_ID,
      function_name: "transfer",
      cpu_insn: 90_000,
      resource_fee_charged: 800,
      ledger: 120_399,
    }),
  ],
  next_cursor: "cursor-2",
};

const SECOND_PAGE = {
  invocations: [
    invocation({
      tx_hash:
        "c1c2d3e4f5061728394a5b6c7d8e9f01c1c2d3e4f5061728394a5b6c7d8e9f01",
      function_name: "mint",
      cpu_insn: 42_000,
      ledger: 120_398,
    }),
  ],
  next_cursor: "",
};

/** Records every /invocations query so a test can assert what the page sent. */
function trackRequests(): URLSearchParams[] {
  return [];
}

function invocationsHandler(
  requests: URLSearchParams[],
  body: (url: URL) => { invocations: unknown[]; next_cursor: string }
): ApiHandler {
  return (route: Route) => {
    const url = new URL(route.request().url());
    requests.push(url.searchParams);
    return { status: 200, body: body(url) };
  };
}

test("/invocations renders invocations from the mocked API", async ({
  page,
}) => {
  const requests = trackRequests();
  await mockApi(page, {
    "api/v1/invocations": invocationsHandler(requests, () => FIRST_PAGE),
  });
  await page.goto("/invocations");

  await expectHeading(page, "Invocations");
  const table = page.getByRole("table");
  for (const item of FIRST_PAGE.invocations) {
    await expect(
      table.getByText(item.function_name, { exact: true })
    ).toBeVisible();
  }
  await expect(
    table.getByText(FIRST_PAGE.invocations[0].cpu_insn.toLocaleString("en-US"))
  ).toBeVisible();
  await expect(page.getByText("Page 1")).toBeVisible();

  // First load requests the configured page size with no filters.
  expect(requests[0].get("limit")).toBe("20");
  expect(requests[0].get("cursor")).toBeNull();
});

test("/invocations sends every filter and resets to the first page", async ({
  page,
}) => {
  const requests = trackRequests();
  await mockApi(page, {
    "api/v1/invocations": invocationsHandler(requests, () => FIRST_PAGE),
  });
  await page.goto("/invocations?cursor=cursor-old&page=3");

  await page.getByLabel("Contract ID").fill(SECOND_CONTRACT_ID);
  await page.getByLabel("Function name").fill("transfer");
  await page.getByLabel("From date").fill("2026-09-01");
  await page.getByLabel("To date").fill("2026-09-30");
  await page.getByRole("button", { name: "Apply" }).click();

  await expect(page).toHaveURL(/contract=CBVRQGH5/);
  await expect(page).toHaveURL(/fn=transfer/);
  await expect(page).toHaveURL(/since=2026-09-01/);
  await expect(page).toHaveURL(/until=2026-09-30/);
  // Applying filters starts a new result set, so the cursor and page reset.
  await expect(page).not.toHaveURL(/cursor=/);
  await expect(page).not.toHaveURL(/page=/);

  const sent = requests[requests.length - 1];
  expect(sent.get("contract_id")).toBe(SECOND_CONTRACT_ID);
  expect(sent.get("fn")).toBe("transfer");
  expect(sent.get("since")).toBe("2026-09-01");
  expect(sent.get("until")).toBe("2026-09-30");
  expect(sent.get("cursor")).toBeNull();
});

test("/invocations clears filters and refetches the unfiltered page", async ({
  page,
}) => {
  const requests = trackRequests();
  await mockApi(page, {
    "api/v1/invocations": invocationsHandler(requests, () => FIRST_PAGE),
  });
  await page.goto("/invocations?fn=transfer&contract=" + CONTRACT_ID);

  await expect(page.getByLabel("Function name")).toHaveValue("transfer");
  await page.getByRole("button", { name: "Clear" }).click();

  await expect(page.getByLabel("Function name")).toHaveValue("");
  await expect(page.getByLabel("Contract ID")).toHaveValue("");
  await expect(page).not.toHaveURL(/fn=/);
  const sent = requests[requests.length - 1];
  expect(sent.get("fn")).toBeNull();
  expect(sent.get("contract_id")).toBeNull();
});

test("/invocations paginates forward and back with the cursor", async ({
  page,
}) => {
  const requests = trackRequests();
  await mockApi(page, {
    "api/v1/invocations": invocationsHandler(requests, (url) =>
      url.searchParams.get("cursor") === "cursor-2" ? SECOND_PAGE : FIRST_PAGE
    ),
  });
  await page.goto("/invocations");

  const next = page.locator("#invocations-next-page");
  const prev = page.locator("#invocations-prev-page");
  await expect(prev).toBeDisabled();
  await expect(next).toBeEnabled();

  await next.click();
  await expect(page).toHaveURL(/cursor=cursor-2/);
  await expect(page).toHaveURL(/page=2/);
  await expect(page.getByText("Page 2")).toBeVisible();
  await expect(
    page
      .getByRole("table")
      .getByText(SECOND_PAGE.invocations[0].function_name, { exact: true })
  ).toBeVisible();
  expect(requests[requests.length - 1].get("cursor")).toBe("cursor-2");

  // The second page is the last one, so Next is disabled.
  await expect(next).toBeDisabled();

  await prev.click();
  await expect(page.getByText("Page 1")).toBeVisible();
  await expect(page.locator("#invocations-prev-page")).toBeDisabled();
  expect(requests[requests.length - 1].get("cursor")).toBeNull();
});

test("/invocations shows the unfiltered empty state", async ({ page }) => {
  await mockApi(page, {
    "api/v1/invocations": () => ({
      status: 200,
      body: { invocations: [], next_cursor: "" },
    }),
  });
  await page.goto("/invocations");

  await expect(page.getByText("No invocations indexed yet")).toBeVisible();
  await expect(page.getByRole("table")).toBeHidden();
});

test("/invocations shows a filtered empty state", async ({ page }) => {
  await mockApi(page, {
    "api/v1/invocations": () => ({
      status: 200,
      body: { invocations: [], next_cursor: "" },
    }),
  });
  await page.goto(`/invocations?fn=does-not-exist`);

  await expect(
    page.getByText("No invocations match your filters")
  ).toBeVisible();
  await expect(page.getByText("No invocations indexed yet")).toBeHidden();
});

test("/invocations shows an error with a retry when the API is unavailable", async ({
  page,
}) => {
  await blockApi(page);
  await page.goto("/invocations");

  await expectHeading(page, "Invocations");
  await expect(page.getByText("Couldn't load invocations.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Try again" })).toBeVisible();
  await expect(page.getByText("No invocations indexed yet")).toBeHidden();
});

test("/invocations restores filters and the cursor from a deep link", async ({
  page,
}) => {
  const requests = trackRequests();
  await mockApi(page, {
    "api/v1/invocations": invocationsHandler(requests, () => SECOND_PAGE),
  });
  await page.goto(
    `/invocations?contract=${SECOND_CONTRACT_ID}&fn=transfer&since=2026-09-01` +
      `&until=2026-09-30&page=2&cursor=cursor-2`
  );

  await expect(page.getByLabel("Contract ID")).toHaveValue(SECOND_CONTRACT_ID);
  await expect(page.getByLabel("Function name")).toHaveValue("transfer");
  await expect(page.getByLabel("From date")).toHaveValue("2026-09-01");
  await expect(page.getByLabel("To date")).toHaveValue("2026-09-30");
  await expect(page.getByText("Page 2")).toBeVisible();

  // The shared link loads that exact page instead of restarting from page one.
  const sent = requests[0];
  expect(sent.get("cursor")).toBe("cursor-2");
  expect(sent.get("contract_id")).toBe(SECOND_CONTRACT_ID);
  expect(sent.get("fn")).toBe("transfer");
});
