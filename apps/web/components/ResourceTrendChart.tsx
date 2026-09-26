"use client";

import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import type { ResourceTrendPoint } from "@/lib/types";

interface ResourceTrendChartProps {
  data: ResourceTrendPoint[];
}

/**
 * Avg CPU / memory / fee per invocation per day over the last 30 days.
 * Each series gets its own Y axis because the units differ by orders of
 * magnitude (instructions vs bytes vs stroops).
 */
export function ResourceTrendChart({ data }: ResourceTrendChartProps) {
  if (data.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center text-sm text-[var(--color-text-secondary)]">
        No resource usage data yet
      </div>
    );
  }

  return (
    <ResponsiveContainer width="100%" height={288}>
      <LineChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
        <XAxis
          dataKey="date"
          tick={{ fontSize: 11, fill: "var(--color-text-secondary)" }}
          tickLine={false}
          axisLine={false}
        />
        <YAxis
          yAxisId="cpu"
          tick={{ fontSize: 11, fill: "var(--color-text-secondary)" }}
          tickLine={false}
          axisLine={false}
          width={48}
        />
        <YAxis yAxisId="mem" hide domain={["auto", "auto"]} />
        <YAxis yAxisId="fee" hide domain={["auto", "auto"]} />
        <Tooltip
          contentStyle={{
            background: "var(--color-bg-card)",
            border: "1px solid var(--color-border)",
            borderRadius: "0.5rem",
            fontSize: "0.875rem",
          }}
        />
        <Legend wrapperStyle={{ fontSize: "0.75rem" }} />
        <Line
          yAxisId="cpu"
          type="monotone"
          dataKey="avg_cpu_insn"
          name="Avg CPU (insn)"
          stroke="var(--color-accent)"
          strokeWidth={2}
          dot={false}
        />
        <Line
          yAxisId="mem"
          type="monotone"
          dataKey="avg_mem_byte"
          name="Avg memory (bytes)"
          stroke="var(--color-safe)"
          strokeWidth={2}
          dot={false}
        />
        <Line
          yAxisId="fee"
          type="monotone"
          dataKey="avg_fee"
          name="Avg fee"
          stroke="var(--color-warning)"
          strokeWidth={2}
          dot={false}
        />
      </LineChart>
    </ResponsiveContainer>
  );
}
