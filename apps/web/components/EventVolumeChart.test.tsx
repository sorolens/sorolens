/**
 * Tests for apps/web/components/EventVolumeChart.tsx (issue #627).
 *
 * vitest + testing-library + jsdom. recharts is mocked: ResponsiveContainer
 * needs real layout to render, so we assert on the props passed to the chart
 * primitives instead (same convention as ResourceTrendChart.test.tsx).
 */

// Registers jest-dom matchers with vitest, including their types.
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { EventVolumeChart } from "./EventVolumeChart";
import type { VolumePoint } from "@/lib/types";

vi.mock("recharts", () => ({
  AreaChart: ({
    data,
    children,
  }: {
    data?: unknown[];
    children?: React.ReactNode;
  }) => (
    // Render inside <svg> so the gradient markup the component passes as
    // children does not trip React's unknown-tag warnings in jsdom.
    <div data-testid="area-chart" data-points={data?.length ?? 0}>
      <svg>{children}</svg>
    </div>
  ),
  Area: ({ dataKey, fill }: { dataKey?: string; fill?: string }) => (
    <div data-testid="area" data-key={dataKey} data-fill={fill} />
  ),
  ResponsiveContainer: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="container">{children}</div>
  ),
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
}));

const POINT: VolumePoint = { date: "2026-07-03", ledger: 120_400, count: 6 };

describe("EventVolumeChart", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders the empty state when there is no data", () => {
    render(<EventVolumeChart data={[]} />);

    expect(screen.getByText("No event volume data")).toBeVisible();
    expect(screen.queryByTestId("area-chart")).not.toBeInTheDocument();
  });

  it("renders an area series over the volume points", () => {
    render(
      <EventVolumeChart data={[POINT, { ...POINT, date: "2026-07-04" }]} />
    );

    expect(screen.getByTestId("area-chart")).toHaveAttribute(
      "data-points",
      "2"
    );
    expect(screen.queryByText("No event volume data")).not.toBeInTheDocument();

    const area = screen.getByTestId("area");
    expect(area).toHaveAttribute("data-key", "count");
    expect(area).toHaveAttribute("data-fill", "url(#eventVolumeGradient)");
  });
});
