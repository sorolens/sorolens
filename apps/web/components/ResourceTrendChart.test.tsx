/**
 * Tests for apps/web/components/ResourceTrendChart.tsx (issue #184).
 *
 * vitest + testing-library + jsdom. recharts is mocked: ResponsiveContainer
 * needs real layout to render, so we assert on the props passed to the chart
 * primitives instead.
 */

// Registers jest-dom matchers with vitest, including their types.
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ResourceTrendChart } from "./ResourceTrendChart";
import type { ResourceTrendPoint } from "@/lib/types";

vi.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="container">{children}</div>
  ),
  LineChart: ({
    data,
    children,
  }: {
    data?: unknown[];
    children?: React.ReactNode;
  }) => (
    <div data-testid="line-chart" data-points={data?.length ?? 0}>
      {children}
    </div>
  ),
  Line: ({ dataKey, name }: { dataKey?: string; name?: string }) => (
    <div data-testid="line" data-key={dataKey} data-name={name} />
  ),
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
  Legend: () => null,
}));

const POINT: ResourceTrendPoint = {
  date: "2026-07-03",
  avg_cpu_insn: 2000,
  avg_mem_byte: 600,
  avg_fee: 150,
  count: 2,
};

describe("ResourceTrendChart", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders the empty state when there is no data", () => {
    render(<ResourceTrendChart data={[]} />);

    expect(screen.getByText("No resource usage data yet")).toBeVisible();
    expect(screen.queryByTestId("line-chart")).not.toBeInTheDocument();
  });

  it("renders one line per metric with the trend data", () => {
    render(
      <ResourceTrendChart data={[POINT, { ...POINT, date: "2026-07-04" }]} />
    );

    const chart = screen.getByTestId("line-chart");
    expect(chart).toHaveAttribute("data-points", "2");
    expect(
      screen.queryByText("No resource usage data yet")
    ).not.toBeInTheDocument();

    const lines = screen.getAllByTestId("line");
    expect(lines.map((l) => l.getAttribute("data-key"))).toEqual([
      "avg_cpu_insn",
      "avg_mem_byte",
      "avg_fee",
    ]);
  });
});
