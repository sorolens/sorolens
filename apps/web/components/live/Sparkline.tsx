"use client";

/**
 * One contract's events-per-minute sparkline.
 *
 * Acceptance criterion: "Sparklines redraw smoothly (no full re-render
 * flicker)." Two things make that true here:
 *
 *  1. The chart is never unmounted. Only its `data` prop changes, so the SVG
 *     element and its path are patched in place instead of being torn down and
 *     rebuilt on every 5-second poll.
 *  2. Animations are disabled. Recharts animates a line from its previous
 *     points by default, which is what produces the visible "redraw" on each
 *     update; with `isAnimationActive={false}` the path is simply repainted.
 *
 * The series length is fixed by the API (`per_minute` always has exactly
 * `minutes` buckets), so the x-axis does not rescale between polls either.
 */

import { Line, LineChart, ResponsiveContainer } from "recharts";

export interface SparklineProps {
  /** Per-minute buckets, oldest first. */
  data: number[];
  stroke?: string;
  height?: number;
  /** Accessible description; the chart itself is decorative. */
  label: string;
  testId?: string;
}

/**
 * Picks a colour for a sparkline by share of the window's hottest contract, so
 * a wall display can be read from across the room.
 */
export function sparklineColor(total: number, hottest: number): string {
  if (hottest <= 0 || total <= 0) return "var(--color-text-secondary)";
  const share = total / hottest;
  if (share >= 0.66) return "var(--color-danger)";
  if (share >= 0.33) return "var(--color-warning)";
  return "var(--color-safe)";
}

export function Sparkline({
  data,
  stroke = "var(--color-accent)",
  height = 36,
  label,
  testId = "sparkline",
}: SparklineProps) {
  const points = data.map((value, index) => ({ index, value }));

  return (
    <div
      style={{ height }}
      role="img"
      aria-label={label}
      data-testid={testId}
      className="w-full"
    >
      <ResponsiveContainer width="100%" height="100%">
        <LineChart
          data={points}
          margin={{ top: 3, right: 3, bottom: 3, left: 3 }}
        >
          <Line
            type="monotone"
            dataKey="value"
            stroke={stroke}
            strokeWidth={1.5}
            dot={false}
            isAnimationActive={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
