import { expect, test, type Page } from "@playwright/test";
import { blockApi, expectHeading, mockApi, TRACE_TX_HASH } from "./helpers";

/**
 * End-to-end coverage for `/playground` (issue #626).
 *
 * The page is fully client-side: it renders its endpoint tree from the
 * generated OpenAPI document and sends requests directly to the API origin
 * (`NEXT_PUBLIC_API_URL`, http://localhost:8080 under the Playwright web
 * server). Those requests still match `mockApi`'s catch-all api/v1 route,
 * which matches on the full URL regardless of the port, so no live backend is
 * needed.
 *
 * The first endpoint of the generated document —
 * `GET /api/v1/invocations/{tx_hash}/trace` — is the default selection.
 * `GET /health` is the first parameterless endpoint.
 */

const SETTINGS_KEY = "sorolens_settings";
const DEFAULT_ENDPOINT = "endpoint-GET-/api/v1/invocations/{tx_hash}/trace";

function curlPreview(page: Page) {
  return page.getByTestId("curl-command");
}

function requestPanel(page: Page) {
  // The Request <section> is identified by its heading.
  return page
    .getByRole("heading", { name: "Request", exact: true })
    .locator("..");
}

function responseSection(page: Page) {
  // The Response <section> is identified by its heading; scoping keeps
  // assertions unique even when the layout renders its own live regions.
  return page
    .getByRole("heading", { name: "Response", exact: true })
    .locator("..");
}

test("renders the endpoint tree, request form and curl preview", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/playground");
  await expectHeading(page, "API Playground");

  // Endpoint tree: every group from the OpenAPI document is listed.
  await expect(page.getByTestId(DEFAULT_ENDPOINT)).toBeVisible();
  await expect(page.getByTestId("endpoint-GET-/health")).toBeVisible();
  await expect(page.getByTestId("endpoint-POST-/graphql")).toBeVisible();

  // The default endpoint has one required path parameter, shown in the
  // request panel (the path text also appears in the endpoint tree).
  await expect(
    requestPanel(page).getByText("GET /api/v1/invocations/{tx_hash}/trace")
  ).toBeVisible();
  const pathParam = page.locator("#param-tx_hash");
  await expect(pathParam).toBeVisible();
  await expect(pathParam).toHaveValue("");

  // No request has been sent yet, so the viewer shows its hint.
  await expect(
    page.getByText("Send a request to see the status, headers, and body.")
  ).toBeVisible();

  // The curl preview reflects the default (empty) path parameter.
  await expect(curlPreview(page)).toContainText("curl -X GET");
  await expect(curlPreview(page)).toContainText(
    "http://localhost:8080/api/v1/invocations//trace"
  );
  await expect(curlPreview(page)).not.toContainText("Authorization");
});

test("fills path parameters and API key into the curl preview", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/playground");
  await expectHeading(page, "API Playground");

  await page.locator("#param-tx_hash").fill(TRACE_TX_HASH);
  await expect(curlPreview(page)).toContainText(
    `http://localhost:8080/api/v1/invocations/${TRACE_TX_HASH}/trace`
  );

  await page.getByLabel("API key (optional)").fill("sl_live_abc123");
  await expect(curlPreview(page)).toContainText(
    "-H 'Authorization: Bearer sl_live_abc123'"
  );
});

test("pre-fills the API key saved in Settings", async ({ page }) => {
  await page.addInitScript((key) => {
    window.localStorage.setItem(
      key,
      JSON.stringify({ apiKey: "sl_live_saved_key" })
    );
  }, SETTINGS_KEY);

  await mockApi(page);
  await page.goto("/playground");
  await expectHeading(page, "API Playground");

  await expect(page.getByLabel("API key (optional)")).toHaveValue(
    "sl_live_saved_key"
  );
  await expect(curlPreview(page)).toContainText(
    "-H 'Authorization: Bearer sl_live_saved_key'"
  );
});

test("sends a request and renders status, headers and body", async ({
  page,
}) => {
  // Answer the trace endpoint with a valid payload; the playground hits it on
  // the API origin but mockApi intercepts by path regardless of the port.
  await mockApi(page, {
    "invocations/": () => ({ status: 200, body: { tx_hash: TRACE_TX_HASH } }),
  });
  await page.goto("/playground");
  await expectHeading(page, "API Playground");

  await page.locator("#param-tx_hash").fill(TRACE_TX_HASH);
  await page.locator("#playground-send").click();

  await expect(page.getByTestId("response-status")).toContainText("200");
  await expect(page.getByTestId("response-body")).toContainText(TRACE_TX_HASH);
  await expect(
    responseSection(page).locator("pre").filter({ hasText: "content-type" })
  ).toContainText("application/json");
  // Duration is reported next to the status pill.
  await expect(page.getByText(/^\d+ ms$/)).toBeVisible();
  await expect(
    page.getByText("Send a request to see the status, headers, and body.")
  ).toHaveCount(0);
});

test("renders a non-2xx status in the response badge", async ({ page }) => {
  await mockApi(page, {
    "invocations/": () => ({
      status: 500,
      body: { error: "boom" },
    }),
  });
  await page.goto("/playground");
  await expectHeading(page, "API Playground");

  await page.locator("#playground-send").click();

  await expect(page.getByTestId("response-status")).toContainText("500");
  await expect(page.getByTestId("response-body")).toContainText("boom");
});

test("shows an error in the response panel when the request fails", async ({
  page,
}) => {
  await blockApi(page);
  await page.goto("/playground");
  await expectHeading(page, "API Playground");

  await page.locator("#param-tx_hash").fill(TRACE_TX_HASH);
  await page.locator("#playground-send").click();

  await expect(responseSection(page).getByRole("alert")).toBeVisible();
  // No response was rendered for the failed request.
  await expect(page.getByTestId("response-status")).toHaveCount(0);
});

test("selecting another endpoint resets the form and response", async ({
  page,
}) => {
  await mockApi(page);
  await page.goto("/playground");
  await expectHeading(page, "API Playground");

  // Send a request against the default endpoint first.
  await page.locator("#param-tx_hash").fill(TRACE_TX_HASH);
  await page.locator("#playground-send").click();
  await expect(page.getByTestId("response-status")).toContainText("200");

  // Switching endpoints clears the response and shows the parameterless hint.
  await page.getByTestId("endpoint-GET-/health").click();
  await expect(
    page.getByText("This endpoint has no parameters.")
  ).toBeVisible();
  await expect(page.getByTestId("response-status")).toHaveCount(0);
  await expect(
    page.getByText("Send a request to see the status, headers, and body.")
  ).toBeVisible();

  // Endpoints with a request body get the JSON editor.
  await page.getByTestId("endpoint-POST-/graphql").click();
  await expect(page.getByText("Request body (JSON)")).toBeVisible();
  await expect(page.locator("#request-body")).toHaveValue("{}");
});
