import { cleanup, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, expect, it, vi } from "vitest";
import { getContractUpgrades } from "@/lib/api";
import { HealthScoreHistory } from "./HealthScoreHistory";

vi.mock("@/lib/api", () => ({
  getContractUpgrades: vi.fn(),
}));

vi.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children: ReactNode }) => children,
  LineChart: ({ children, data }: { children: ReactNode; data: unknown }) => (
    <div data-testid="history-chart" data-points={JSON.stringify(data)}>
      {children}
    </div>
  ),
  ReferenceLine: ({ x }: { x: number }) => (
    <span data-testid="upgrade-marker" data-time={x} />
  ),
  CartesianGrid: () => null,
  Line: () => null,
  Tooltip: () => null,
  XAxis: () => null,
  YAxis: () => null,
}));

afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});

it("explains why a single score cannot show a trend", () => {
  render(
    <HealthScoreHistory
      contractId="C1"
      history={[{ computed_at: "2026-07-03T08:00:00Z", score: 80 }]}
    />
  );

  expect(
    screen.getByText("At least two recorded scores are needed to show a trend.")
  ).toBeVisible();
  expect(screen.queryByTestId("history-chart")).toBeNull();
  expect(getContractUpgrades).not.toHaveBeenCalled();
});

it("leaves a gap for a missing hour and marks upgrades in the chart range", async () => {
  const first = "2026-07-03T08:00:00Z";
  const missing = "2026-07-03T09:00:00Z";
  const upgrade = "2026-07-03T09:30:00Z";
  const last = "2026-07-03T10:00:00Z";

  vi.mocked(getContractUpgrades).mockResolvedValue({
    upgrades: [
      { ledger: 1, at: "2026-07-03T07:00:00Z" },
      { ledger: 2, at: upgrade },
    ],
  });

  render(
    <HealthScoreHistory
      contractId="C1"
      history={[
        { computed_at: first, score: 80 },
        { computed_at: last, score: 90 },
      ]}
    />
  );

  expect(screen.getByTestId("history-chart")).toHaveAttribute(
    "data-points",
    JSON.stringify([
      { time: Date.parse(first), score: 80 },
      { time: Date.parse(missing), score: null },
      { time: Date.parse(last), score: 90 },
    ])
  );

  const markers = await screen.findAllByTestId("upgrade-marker");
  expect(markers).toHaveLength(1);
  expect(markers[0]).toHaveAttribute("data-time", String(Date.parse(upgrade)));
});
