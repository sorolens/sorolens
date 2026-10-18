"use client";

import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import type { InvocationFrequencyPoint } from "@/lib/types";

interface InvocationFrequencyChartProps {
  data: InvocationFrequencyPoint[];
}

/** Tooltip text: exact invocation count for the hovered hour. */
export function formatCountTooltip(
  value: number | string | (number | string)[]
): string {
  const count = Array.isArray(value) ? value[0] : value;
  return `${count} invocation${Number(count) === 1 ? "" : "s"}`;
}

/**
 * Histogram of invocation counts per hour over the last 24 hours. Hovering
 * a bar shows the exact count via the tooltip (issue #185).
 */
export function InvocationFrequencyChart({
  data,
}: InvocationFrequencyChartProps) {
  if (data.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center text-sm text-[var(--color-text-secondary)]">
        No invocation data yet
      </div>
    );
  }

  return (
    <ResponsiveContainer width="100%" height={256}>
      <BarChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
        <XAxis
          dataKey="hour"
          tick={{ fontSize: 11, fill: "var(--color-text-secondary)" }}
          tickLine={false}
          axisLine={false}
          interval="preserveStartEnd"
        />
        <YAxis
          allowDecimals={false}
          tick={{ fontSize: 11, fill: "var(--color-text-secondary)" }}
          tickLine={false}
          axisLine={false}
          width={40}
        />
        <Tooltip
          contentStyle={{
            background: "var(--color-bg-card)",
            border: "1px solid var(--color-border)",
            borderRadius: "0.5rem",
            fontSize: "0.875rem",
          }}
          formatter={(value) => formatCountTooltip(value)}
        />
        <Bar
          dataKey="count"
          name="Invocations"
          fill="var(--color-accent)"
          radius={[4, 4, 0, 0]}
          opacity={0.8}
        />
      </BarChart>
    </ResponsiveContainer>
  );
}
