"use client";

import { Area, AreaChart, ResponsiveContainer } from "recharts";
import type { CompareVolumePoint } from "@/lib/types";

interface EventVolumeSparklineProps {
  data: CompareVolumePoint[];
  height?: number;
}

/**
 * Compact event-volume trend for one comparison column. Recharts is already a
 * dashboard dependency, so this is a bare area chart with no axes or tooltip —
 * the numeric value lives in the metric rows beside it.
 */
export function EventVolumeSparkline({
  data,
  height = 56,
}: EventVolumeSparklineProps) {
  const hasSignal = data.length > 0 && data.some((p) => p.count > 0);
  if (!hasSignal) {
    return (
      <div
        className="flex h-14 items-center justify-center rounded-md bg-[var(--color-bg-page)] text-[10px] text-[var(--color-text-secondary)]"
        data-testid="sparkline-empty"
      >
        No event volume yet
      </div>
    );
  }

  return (
    <div
      className="rounded-md bg-[var(--color-bg-page)] pt-1"
      data-testid="sparkline"
    >
      <ResponsiveContainer width="100%" height={height}>
        <AreaChart
          data={data}
          margin={{ top: 2, right: 0, bottom: 0, left: 0 }}
        >
          <Area
            type="monotone"
            dataKey="count"
            stroke="var(--color-accent)"
            fill="var(--color-accent)"
            fillOpacity={0.15}
            strokeWidth={1.5}
            isAnimationActive={false}
            dot={false}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
