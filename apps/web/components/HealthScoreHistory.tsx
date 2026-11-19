"use client";

import { useEffect, useState } from "react";
import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { getContractUpgrades } from "@/lib/api";
import type { ContractUpgradeMarker, HealthScoreResponse } from "@/lib/types";

const HOUR_MS = 60 * 60 * 1000;

const tickDate = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
  hour: "numeric",
  minute: "2-digit",
  timeZone: "UTC",
});

interface HealthScoreHistoryProps {
  contractId: string;
  history: NonNullable<HealthScoreResponse["history"]>;
}

export function HealthScoreHistory({
  contractId,
  history,
}: HealthScoreHistoryProps) {
  const [upgrades, setUpgrades] = useState<ContractUpgradeMarker[]>([]);
  const [upgradesUnavailable, setUpgradesUnavailable] = useState(false);

  const samples = history
    .map(({ computed_at, score }) => ({ time: Date.parse(computed_at), score }))
    .filter(
      ({ time, score }) => Number.isFinite(time) && Number.isFinite(score)
    )
    .sort((a, b) => a.time - b.time);
  const hasTrend = samples.length >= 2;

  useEffect(() => {
    if (!hasTrend) return;

    let cancelled = false;
    getContractUpgrades(contractId)
      .then(({ upgrades }) => {
        if (!cancelled) setUpgrades(upgrades);
      })
      .catch(() => {
        if (!cancelled) setUpgradesUnavailable(true);
      });

    return () => {
      cancelled = true;
    };
  }, [contractId, hasTrend]);

  const points: { time: number; score: number | null }[] = [];
  for (const sample of samples) {
    const previous = points[points.length - 1];
    if (
      previous &&
      Math.floor(sample.time / HOUR_MS) - Math.floor(previous.time / HOUR_MS) >
        1
    ) {
      points.push({ time: previous.time + HOUR_MS, score: null });
    }
    points.push(sample);
  }

  const first = samples[0]?.time;
  const last = samples[samples.length - 1]?.time;
  const markers = upgrades
    .map((upgrade) => ({ ...upgrade, time: Date.parse(upgrade.at) }))
    .filter(({ time }) => first <= time && time <= last);

  return (
    <section
      aria-label="Health score history"
      className="mt-4 border-t border-[var(--color-border)] pt-4"
    >
      <h3 className="text-sm font-medium">Health score history</h3>
      {!hasTrend ? (
        <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
          At least two recorded scores are needed to show a trend.
        </p>
      ) : (
        <>
          <p className="mt-1 text-xs text-[var(--color-text-secondary)]">
            Times are UTC. Gaps indicate hours without a recorded score.
          </p>
          <div className="mt-2 min-w-0">
            <ResponsiveContainer width="100%" height={240}>
              <LineChart data={points} margin={{ top: 16, right: 16, left: 0 }}>
                <CartesianGrid
                  stroke="var(--color-border)"
                  strokeDasharray="3 3"
                />
                <XAxis
                  dataKey="time"
                  type="number"
                  domain={[first, last]}
                  tickFormatter={(value: number) => tickDate.format(value)}
                  tick={{ fontSize: 11 }}
                />
                <YAxis domain={[0, 100]} width={32} tick={{ fontSize: 11 }} />
                <Tooltip
                  labelFormatter={(value) =>
                    `${tickDate.format(Number(value))} UTC`
                  }
                />
                {markers.map(({ ledger, time }) => (
                  <ReferenceLine
                    key={`${ledger}-${time}`}
                    x={time}
                    stroke="var(--color-warning)"
                    strokeDasharray="4 3"
                  />
                ))}
                <Line
                  dataKey="score"
                  name="Health score"
                  stroke="var(--color-accent)"
                  strokeWidth={2}
                  dot={{ r: 2 }}
                  connectNulls={false}
                  isAnimationActive={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
          {markers.length > 0 && (
            <p className="text-xs text-[var(--color-text-secondary)]">
              Dashed vertical lines mark contract upgrades.
            </p>
          )}
          {upgradesUnavailable && (
            <p className="text-xs text-[var(--color-text-secondary)]">
              Upgrade markers are unavailable.
            </p>
          )}
        </>
      )}
    </section>
  );
}
