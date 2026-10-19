/**
 * Tests for apps/web/components/InvocationChart.tsx (issue #627).
 *
 * vitest + testing-library + jsdom. recharts is mocked: ResponsiveContainer
 * needs real layout to render, so we assert on the props passed to the chart
 * primitives instead (same convention as ResourceTrendChart.test.tsx).
 */

// Registers jest-dom matchers with vitest, including their types.
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { InvocationChart } from "./InvocationChart";
import type { VolumePoint } from "@/lib/types";

vi.mock("recharts", () => ({
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
  Bar: ({ dataKey, fill }: { dataKey?: string; fill?: string }) => (
    <div data-testid="bar" data-key={dataKey} data-fill={fill} />
  ),
  ResponsiveContainer: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="container">{children}</div>
  ),
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
}));

const POINT: VolumePoint = { date: "2026-07-03", ledger: 120_400, count: 2 };

describe("InvocationChart", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders the empty state when there is no data", () => {
    render(<InvocationChart data={[]} />);

    expect(screen.getByText("No invocation data")).toBeVisible();
    expect(screen.queryByTestId("bar-chart")).not.toBeInTheDocument();
  });

  it("renders a bar series over the invocation points", () => {
    render(
      <InvocationChart data={[POINT, { ...POINT, date: "2026-07-04" }]} />
    );

    expect(screen.getByTestId("bar-chart")).toHaveAttribute("data-points", "2");
    expect(screen.queryByText("No invocation data")).not.toBeInTheDocument();

    const bar = screen.getByTestId("bar");
    expect(bar).toHaveAttribute("data-key", "count");
    expect(bar).toHaveAttribute("data-fill", "var(--color-accent)");
  });
});
