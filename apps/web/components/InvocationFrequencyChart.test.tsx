/**
 * Tests for apps/web/components/InvocationFrequencyChart.tsx (issue #185).
 *
 * vitest + testing-library + jsdom. recharts is mocked: ResponsiveContainer
 * needs real layout to render, so we assert on the props passed to the chart
 * primitives instead.
 */

// Registers jest-dom matchers with vitest, including their types.
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  InvocationFrequencyChart,
  formatCountTooltip,
} from "./InvocationFrequencyChart";
import type { InvocationFrequencyPoint } from "@/lib/types";

vi.mock("recharts", () => ({
  ResponsiveContainer: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="container">{children}</div>
  ),
  BarChart: ({
    data,
    children,
  }: {
    data?: unknown[];
    children?: React.ReactNode;
  }) => (
    <div data-testid="bar-chart" data-points={data?.length ?? 0}>
      {children}
    </div>
  ),
  Bar: ({ dataKey, name }: { dataKey?: string; name?: string }) => (
    <div data-testid="bar" data-key={dataKey} data-name={name} />
  ),
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
}));

const POINTS: InvocationFrequencyPoint[] = [
  { hour: "09:00", count: 2 },
  { hour: "10:00", count: 1 },
];

describe("InvocationFrequencyChart", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders the empty state when there is no data", () => {
    render(<InvocationFrequencyChart data={[]} />);

    expect(screen.getByText("No invocation data yet")).toBeVisible();
    expect(screen.queryByTestId("bar-chart")).not.toBeInTheDocument();
  });

  it("renders a bar chart over the hourly buckets", () => {
    render(<InvocationFrequencyChart data={POINTS} />);

    const chart = screen.getByTestId("bar-chart");
    expect(chart).toHaveAttribute("data-points", "2");
    expect(screen.queryByText("No invocation data yet")).not.toBeInTheDocument();

    const bar = screen.getByTestId("bar");
    expect(bar).toHaveAttribute("data-key", "count");
    expect(bar).toHaveAttribute("data-name", "Invocations");
  });

  describe("formatCountTooltip", () => {
    it("shows the exact count with singular/plural wording", () => {
      expect(formatCountTooltip(0)).toBe("0 invocations");
      expect(formatCountTooltip(1)).toBe("1 invocation");
      expect(formatCountTooltip(42)).toBe("42 invocations");
      // Values arrive from recharts as strings.
      expect(formatCountTooltip("7")).toBe("7 invocations");
    });
  });
});
