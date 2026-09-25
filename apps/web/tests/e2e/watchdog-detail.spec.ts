import { test, expect } from "@playwright/test";
import {
  CONTRACT_ID,
  expectHeading,
  mockApi,
  watchdogDetailHandlers,
} from "./helpers";

/**
 * Health checks come back newest first from
 * GET /api/v1/watchdog/contracts/{id}/health, exactly as the API orders them.
 * Read oldest -> newest this is Healthy, Healthy, Degraded, Unresponsive,
 * Healthy: four runs and three transitions.
 */
const HEALTH_CHECKS = [
  {
    contract_id: CONTRACT_ID,
    status: "Healthy",
    metadata: "ok",
    ledger: 120_405,
    tx_hash: "e5",
    timestamp: "2026-07-03T08:00:00Z",
  },
  {
    contract_id: CONTRACT_ID,
    status: "Unresponsive",
    metadata: "timeout",
    ledger: 120_404,
    tx_hash: "e4",
    timestamp: "2026-07-02T08:00:00Z",
  },
  {
    contract_id: CONTRACT_ID,
    status: "Degraded",
    metadata: "slow response",
    ledger: 120_403,
    tx_hash: "e3",
    timestamp: "2026-07-01T08:00:00Z",
  },
  {
    contract_id: CONTRACT_ID,
    status: "Healthy",
    metadata: "ok",
    ledger: 120_402,
    tx_hash: "e2",
    timestamp: "2026-06-30T08:00:00Z",
  },
  {
    contract_id: CONTRACT_ID,
    status: "Healthy",
    metadata: "ok",
    ledger: 120_401,
    tx_hash: "e1",
    timestamp: "2026-06-29T08:00:00Z",
  },
];

test("/watchdog/[id] renders a status timeline with a colour legend", async ({
  page,
}) => {
  await mockApi(page, watchdogDetailHandlers(HEALTH_CHECKS));
  await page.goto(`/watchdog/${CONTRACT_ID}`);

  await expectHeading(page, "Escrow DEX");
  await expect(page.getByTestId("watchdog-timeline")).toBeVisible();

  // Consecutive same-status checks collapse into a single segment each.
  const segments = page.getByTestId("timeline-segment");
  await expect(segments).toHaveCount(4);
  await expect(segments.nth(0)).toHaveAttribute("data-status", "Healthy");
  await expect(segments.nth(1)).toHaveAttribute("data-status", "Degraded");
  await expect(segments.nth(2)).toHaveAttribute("data-status", "Unresponsive");
  await expect(segments.nth(3)).toHaveAttribute("data-status", "Healthy");

  // Legend documents every status colour.
  const legendItems = page.getByTestId("timeline-legend-item");
  await expect(legendItems).toHaveCount(3);
  for (const status of ["Healthy", "Degraded", "Unresponsive"]) {
    await expect(legendItems.filter({ hasText: status })).toBeVisible();
  }

  // Every change between two segments is called out below the timeline.
  await expect(page.getByTestId("timeline-transition")).toHaveCount(3);
});
