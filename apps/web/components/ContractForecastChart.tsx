"use client";

import { useEffect, useState } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { getContractForecast } from "@/lib/api";
import type { ContractForecastSeries } from "@/lib/types";

const HORIZONS = [7, 14, 30, 90] as const;
const METRICS = [
  { key: "fees", label: "Fees" },
  { key: "invocations", label: "Invocations" },
  { key: "events", label: "Events" },
] as const;

interface Props {
  contractId: string;
}

export function ContractForecastChart({ contractId }: Props) {
  const [horizon, setHorizon] = useState<number>(30);
  const [metric, setMetric] = useState<(typeof METRICS)[number]["key"]>("fees");
  const [series, setSeries] = useState<ContractForecastSeries | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(false);
    getContractForecast(contractId, horizon)
      .then((response) => {
        if (!cancelled) {
          setSeries(
            response.series?.find((item) => item.metric === metric) ?? null
          );
        }
      })
      .catch(() => {
        if (!cancelled) {
          setSeries(null);
          setError(true);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [contractId, horizon, metric]);

  const selectedMetric = METRICS.find((item) => item.key === metric)!;
  const chartData = series?.points.map((point) => ({
    ...point,
    confidence: Math.max(0, point.upper - point.lower),
  }));

  return (
    <section aria-labelledby="forecast-heading" className="mb-8">
      <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 id="forecast-heading" className="text-xl font-semibold">
            Cost forecast
          </h2>
          <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
            Daily projection with a 95% confidence range, based on the last 90
            days.
          </p>
        </div>
        <label className="flex items-center gap-2 text-sm text-[var(--color-text-secondary)]">
          Horizon
          <select
            aria-label="Forecast horizon"
            value={horizon}
            onChange={(event) => setHorizon(Number(event.target.value))}
            className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2 text-[var(--color-text-primary)]"
          >
            {HORIZONS.map((days) => (
              <option key={days} value={days}>
                {days} days
              </option>
            ))}
          </select>
        </label>
      </div>

      <div
        className="mb-3 inline-flex border-b border-[var(--color-border)]"
        role="tablist"
        aria-label="Forecast metric"
      >
        {METRICS.map((item) => (
          <button
            key={item.key}
            type="button"
            role="tab"
            aria-selected={metric === item.key}
            onClick={() => setMetric(item.key)}
            className={`border-b-2 px-3 py-2 text-sm ${metric === item.key ? "border-[var(--color-accent)] text-[var(--color-text-primary)]" : "border-transparent text-[var(--color-text-secondary)]"}`}
          >
            {item.label}
          </button>
        ))}
      </div>

      {loading ? (
        <p
          className="flex h-64 items-center justify-center text-sm text-[var(--color-text-secondary)]"
          role="status"
          aria-busy="true"
        >
          Loading forecast…
        </p>
      ) : error ? (
        <p
          className="flex h-64 items-center justify-center text-sm text-[var(--color-text-secondary)]"
          role="alert"
        >
          Forecast is temporarily unavailable.
        </p>
      ) : !chartData?.length ? (
        <p className="flex h-64 items-center justify-center text-sm text-[var(--color-text-secondary)]">
          Not enough indexed data to forecast{" "}
          {selectedMetric.label.toLowerCase()} yet.
        </p>
      ) : (
        <div
          role="img"
          aria-label={`${selectedMetric.label} forecast with 95% confidence range`}
          data-testid="forecast-chart"
        >
          <ResponsiveContainer width="100%" height={288}>
            <AreaChart
              data={chartData}
              margin={{ top: 8, right: 12, left: 0, bottom: 0 }}
            >
              <CartesianGrid
                strokeDasharray="3 3"
                stroke="var(--color-border)"
              />
              <XAxis
                dataKey="date"
                tick={{ fontSize: 11, fill: "var(--color-text-secondary)" }}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                tick={{ fontSize: 11, fill: "var(--color-text-secondary)" }}
                tickLine={false}
                axisLine={false}
                width={56}
              />
              <Tooltip
                formatter={(value, name) => [
                  Number(value).toLocaleString(),
                  name,
                ]}
                contentStyle={{
                  background: "var(--color-bg-card)",
                  border: "1px solid var(--color-border)",
                  borderRadius: "0.5rem",
                  fontSize: "0.875rem",
                }}
              />
              <Area
                dataKey="lower"
                stackId="range"
                stroke="none"
                fill="transparent"
                name="Lower bound"
              />
              <Area
                dataKey="confidence"
                stackId="range"
                stroke="none"
                fill="var(--color-safe)"
                fillOpacity={0.2}
                name="95% confidence range"
              />
              <Line
                dataKey="value"
                stroke="var(--color-accent)"
                strokeWidth={2}
                dot={false}
                name={selectedMetric.label}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}
    </section>
  );
}
