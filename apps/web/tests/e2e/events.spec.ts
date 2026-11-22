import { expect, test, type Page } from "@playwright/test";
import {
  CONTRACT_ID,
  contractEvent,
  newerContractEvent,
  mockApi,
} from "./helpers";

type MockEventSource = {
  url: string;
  onopen: (() => void) | null;
  onmessage: ((event: { data: string }) => void) | null;
  onerror: (() => void) | null;
};

declare global {
  interface Window {
    __eventSources: MockEventSource[];
  }
}

async function installMockEventSource(page: Page) {
  await page.addInitScript(() => {
    window.__eventSources = [];

    class MockEventSource {
      onopen: (() => void) | null = null;
      onmessage: ((event: { data: string }) => void) | null = null;
      onerror: (() => void) | null = null;
      url: string;

      constructor(url: string) {
        this.url = url;
        window.__eventSources.push(this);
        queueMicrotask(() => this.onopen?.());
      }

      close() {}
    }

    window.EventSource = MockEventSource as unknown as typeof EventSource;
  });
}

test("filters events and pages through cursor results", async ({ page }) => {
  const requests: URL[] = [];
  await mockApi(page, {
    events: (route) => {
      const url = new URL(route.request().url());
      requests.push(url);
      const secondPage = url.searchParams.has("cursor");
      return {
        status: 200,
        body: {
          events: [secondPage ? newerContractEvent() : contractEvent()],
          next_cursor: secondPage ? "" : "cursor-2",
        },
      };
    },
  });

  await page.goto("/events");
  await expect(page.getByRole("heading", { name: "Events" })).toBeVisible();
  await page.getByLabel("Contract ID").fill(CONTRACT_ID.slice(0, 8));
  await page.getByLabel("Event type").selectOption("contract");
  await page.getByRole("button", { name: "Apply" }).click();

  await expect
    .poll(() =>
      requests.some(
        (url) =>
          url.searchParams.get("contract_id") === CONTRACT_ID.slice(0, 8) &&
          url.searchParams.get("type") === "contract"
      )
    )
    .toBe(true);

  const pagination = page.getByRole("navigation", { name: "Pagination" });
  await pagination.getByRole("button", { name: "Next →" }).click();
  await expect(pagination.getByText("Page 2")).toBeVisible();
  await expect
    .poll(() =>
      requests.some((url) => url.searchParams.get("cursor") === "cursor-2")
    )
    .toBe(true);

  await pagination.getByRole("button", { name: "Previous" }).click();
  await expect(pagination.getByText("Page 1")).toBeVisible();
});

test("shows the empty events state", async ({ page }) => {
  await mockApi(page, {
    events: () => ({ status: 200, body: { events: [], next_cursor: "" } }),
  });

  await page.goto("/events");
  await expect(page.getByText("No events indexed yet.")).toBeVisible();
});

test("shows the events error state", async ({ page }) => {
  await mockApi(page, {
    events: () => ({ status: 500, body: { error: "upstream unavailable" } }),
  });

  await page.goto("/events");
  await expect(page.getByText("Couldn't load events.")).toBeVisible();
});

test("connects to the live SSE stream", async ({ page }) => {
  await installMockEventSource(page);
  await mockApi(page);

  await page.goto("/events/live");
  await expect(page.getByText("Live (SSE)", { exact: true })).toBeVisible();
  await expect
    .poll(() => page.evaluate(() => window.__eventSources.length))
    .toBeGreaterThan(0);
});

test("falls back to polling after repeated stream failures", async ({
  page,
}) => {
  await installMockEventSource(page);
  await mockApi(page);

  await page.goto("/events/live");
  await page.getByPlaceholder("Filter by Contract ID...").fill(CONTRACT_ID);
  await page.getByRole("button", { name: "Filter" }).click();
  await expect(page.getByText("Live (SSE)", { exact: true })).toBeVisible();
  await page.evaluate(() => {
    const source = window.__eventSources.at(-1);
    source?.onerror?.();
    source?.onerror?.();
    source?.onerror?.();
  });

  await expect(
    page.getByText("Live (Polling Fallback)", { exact: true })
  ).toBeVisible();
  await expect(
    page.getByRole("cell", { name: "transfer" }).first()
  ).toBeVisible();
});
