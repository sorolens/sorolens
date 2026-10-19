import { expect, test, type Page, type Route } from "@playwright/test";
import {
  CONTRACT_ID,
  alertSubscription,
  expectHeading,
  mockApi,
  monitoredContract,
} from "./helpers";

/**
 * End-to-end coverage for `/watchdog/notifications` (issue #626).
 *
 * The page mints a workspace session (mocked `POST /api/v1/auth/session`),
 * then lists alert-channel subscriptions from
 * `GET /api/v1/watchdog/subscriptions`. The add form validates client-side
 * with the same rules as the API, and test notifications go through the
 * Next.js route handler `/api/watchdog/test-notification`, which is
 * intercepted separately from the `/api/v1/**` mock.
 */

type SubHandler = (route: Route) => { status: number; body: unknown };

/** Method-aware subscriptions handler: GET lists, POST creates, DELETE drops. */
function subscriptionsHandlers(
  list: SubHandler,
  onCreate?: (route: Route) => { status: number; body: unknown },
  onDelete?: (route: Route) => { status: number; body: unknown }
) {
  return {
    "watchdog/subscriptions": (route: Route) => {
      const method = route.request().method();
      if (method === "POST" && onCreate) return onCreate(route);
      if (method === "DELETE" && onDelete) return onDelete(route);
      return list(route);
    },
  };
}

/** Intercepts the Next.js test-notification proxy route. */
async function mockTestNotification(page: Page, handler: SubHandler) {
  await page.route("**/api/watchdog/test-notification", (route) => {
    const { status, body } = handler(route);
    return route.fulfill({
      status,
      contentType: "application/json",
      body: JSON.stringify(body),
    });
  });
}

async function openContractPicker(page: Page) {
  await page.getByTestId("contract-picker-trigger").click();
  await page.getByRole("option", { name: /Escrow DEX/ }).click();
}

function slackDestination(page: Page) {
  return page.getByLabel("Slack incoming webhook URL");
}

/**
 * Next.js renders an always-present route-announcer with role="alert", so
 * plain getByRole("alert") is never unique. Scope to real page alerts.
 */
function pageAlert(page: Page) {
  return page.locator('[role="alert"]:not([id="__next-route-announcer__"])');
}

test.describe("Watchdog notification channels", () => {
  test("lists active channels", async ({ page }) => {
    await mockApi(
      page,
      subscriptionsHandlers(() => ({
        status: 200,
        body: { subscriptions: [alertSubscription()] },
      }))
    );
    await page.goto("/watchdog/notifications");
    await expectHeading(page, "Notification channels");

    const table = page.getByRole("table");
    // Contract column renders a truncated mono id.
    await expect(
      table.getByText(CONTRACT_ID.slice(0, 6)).first()
    ).toBeVisible();
    await expect(table.getByText("Slack", { exact: true })).toBeVisible();
    await expect(table.getByText("Critical", { exact: true })).toBeVisible();
    await expect(
      table.getByText("https://hooks.slack.com/services/T000/B000/xyz")
    ).toBeVisible();
    await expect(
      table.getByRole("button", { name: "Configure" })
    ).toBeVisible();
    await expect(table.getByRole("button", { name: "Remove" })).toBeVisible();
  });

  test("shows an empty state when there are no channels", async ({ page }) => {
    await mockApi(
      page,
      subscriptionsHandlers(() => ({
        status: 200,
        body: { subscriptions: [] },
      }))
    );
    await page.goto("/watchdog/notifications");
    await expectHeading(page, "Notification channels");

    await expect(page.getByText(/No notification channels yet/)).toBeVisible();
    await expect(page.getByRole("table")).toHaveCount(0);
  });

  test("shows the API error when listing channels fails", async ({ page }) => {
    await mockApi(
      page,
      subscriptionsHandlers(() => ({ status: 500, body: { error: "boom" } }))
    );
    await page.goto("/watchdog/notifications");
    await expectHeading(page, "Notification channels");

    await expect(pageAlert(page)).toContainText("boom");
    await expect(page.getByText(/No notification channels yet/)).toBeHidden();
  });

  test("explains missing contributor access for a 401", async ({ page }) => {
    await mockApi(
      page,
      subscriptionsHandlers(() => ({
        status: 401,
        body: { error: "forbidden" },
      }))
    );
    await page.goto("/watchdog/notifications");
    await expectHeading(page, "Notification channels");

    await expect(pageAlert(page)).toContainText(
      "Notification settings need a contributor account."
    );
  });

  test("validates the add form before contacting the API", async ({ page }) => {
    let posts = 0;
    await mockApi(
      page,
      subscriptionsHandlers(
        () => ({ status: 200, body: { subscriptions: [] } }),
        () => {
          posts += 1;
          return { status: 201, body: alertSubscription() };
        }
      )
    );
    await page.goto("/watchdog/notifications");
    await expectHeading(page, "Notification channels");

    // No contract chosen yet.
    await page.getByRole("button", { name: "Add Slack channel" }).click();
    await expect(pageAlert(page)).toContainText("Choose a contract.");

    await openContractPicker(page);
    await expect(page.getByTestId("contract-picker-clear")).toBeVisible();

    // Contract chosen but no destination.
    await page.getByRole("button", { name: "Add Slack channel" }).click();
    await expect(pageAlert(page)).toContainText(
      "Slack incoming webhook URL is required."
    );

    // Slack requires an https URL.
    await slackDestination(page).fill("http://example.com/hook");
    await page.getByRole("button", { name: "Add Slack channel" }).click();
    await expect(pageAlert(page)).toContainText(
      "Slack incoming webhook URL must be an https URL."
    );

    // Nothing reached the API.
    expect(posts).toBe(0);
  });

  test("switching channel and severity updates labels and helper text", async ({
    page,
  }) => {
    await mockApi(
      page,
      subscriptionsHandlers(() => ({
        status: 200,
        body: { subscriptions: [] },
      }))
    );
    await page.goto("/watchdog/notifications");
    await expectHeading(page, "Notification channels");

    // Defaults: Slack + Critical.
    await expect(page.getByRole("radio", { name: "Slack" })).toBeChecked();
    await expect(page.getByRole("radio", { name: "Critical" })).toBeChecked();
    await expect(
      page.getByText("Deliver only Critical watchdog alerts.")
    ).toBeVisible();

    // The radio inputs are visually hidden (sr-only), so drive them through
    // their visible labels, which carry the click handler.
    await page.getByText("Warning", { exact: true }).click();
    await expect(page.getByRole("radio", { name: "Warning" })).toBeChecked();
    await expect(
      page.getByText("Deliver Warning and Critical watchdog alerts.")
    ).toBeVisible();

    // Switching channels swaps the destination field and submit label.
    await page.getByText("Discord", { exact: true }).click();
    await expect(page.getByRole("radio", { name: "Discord" })).toBeChecked();
    await expect(page.getByLabel("Discord webhook URL")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Add Discord channel" })
    ).toBeVisible();

    await page.getByText("PagerDuty", { exact: true }).click();
    await expect(page.getByRole("radio", { name: "PagerDuty" })).toBeChecked();
    await expect(page.getByLabel("Integration (routing) key")).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Add PagerDuty channel" })
    ).toBeVisible();
  });

  test("adds a channel through the picker and submit", async ({ page }) => {
    let createdBody: Record<string, unknown> | null = null;
    await mockApi(
      page,
      subscriptionsHandlers(
        () => ({ status: 200, body: { subscriptions: [] } }),
        (route) => {
          createdBody = route.request().postDataJSON();
          return {
            status: 201,
            body: alertSubscription({
              webhook_url: "https://hooks.slack.com/services/T0/B0/xyz",
            }),
          };
        }
      )
    );
    await page.goto("/watchdog/notifications");
    await expectHeading(page, "Notification channels");

    await openContractPicker(page);
    await slackDestination(page).fill(
      "https://hooks.slack.com/services/T0/B0/xyz"
    );
    await page.getByRole("button", { name: "Add Slack channel" }).click();

    // The created subscription is prepended to the table.
    const table = page.getByRole("table");
    await expect(table.getByText("Slack", { exact: true })).toBeVisible();
    await expect(
      table.getByText("https://hooks.slack.com/services/T0/B0/xyz")
    ).toBeVisible();

    expect(createdBody).toMatchObject({
      contract_id: CONTRACT_ID,
      channel_type: "slack",
      webhook_url: "https://hooks.slack.com/services/T0/B0/xyz",
      severity_filter: "Critical",
    });
  });

  test("sends a test notification and reports success and failure", async ({
    page,
  }) => {
    let upstreamStatus = 200;
    await mockApi(
      page,
      subscriptionsHandlers(() => ({
        status: 200,
        body: { subscriptions: [] },
      }))
    );
    await mockTestNotification(page, () => ({
      status: upstreamStatus,
      body:
        upstreamStatus < 400
          ? { success: true, message: "Delivered to Slack." }
          : { success: false, error: "upstream boom" },
    }));
    await page.goto("/watchdog/notifications");
    await expectHeading(page, "Notification channels");

    await openContractPicker(page);
    await slackDestination(page).fill(
      "https://hooks.slack.com/services/T0/B0/xyz"
    );

    await page.getByTestId("test-send-btn").click();
    await expect(page.getByTestId("test-feedback")).toContainText(
      "Delivered to Slack."
    );

    upstreamStatus = 502;
    await page.getByTestId("test-send-btn").click();
    await expect(page.getByTestId("test-feedback")).toContainText(
      "upstream boom"
    );
  });

  test("removing a channel deletes it and empties the list", async ({
    page,
  }) => {
    let deletes = 0;
    await mockApi(
      page,
      subscriptionsHandlers(
        () => ({ status: 200, body: { subscriptions: [alertSubscription()] } }),
        undefined,
        () => {
          deletes += 1;
          return { status: 204, body: null };
        }
      )
    );
    await page.goto("/watchdog/notifications");
    await expectHeading(page, "Notification channels");

    await page.getByRole("button", { name: "Remove" }).click();
    await expect(page.getByRole("table")).toHaveCount(0);
    await expect(page.getByText(/No notification channels yet/)).toBeVisible();
    expect(deletes).toBe(1);
  });

  test("Configure reloads a channel's settings into the form", async ({
    page,
  }) => {
    const webhookSub = alertSubscription({
      channel_type: "webhook",
      webhook_url: "https://example.com/sorolens-alerts",
    });
    await mockApi(
      page,
      subscriptionsHandlers(() => ({
        status: 200,
        body: { subscriptions: [webhookSub] },
      }))
    );
    await page.goto("/watchdog/notifications");
    await expectHeading(page, "Notification channels");

    await page.getByRole("button", { name: "Configure" }).click();

    await expect(page.getByRole("radio", { name: "Webhook" })).toBeChecked();
    await expect(page.getByLabel("Webhook URL", { exact: false })).toHaveValue(
      "https://example.com/sorolens-alerts"
    );
    await expect(page.getByTestId("contract-picker-trigger")).toContainText(
      CONTRACT_ID.slice(0, 6)
    );
  });
});
