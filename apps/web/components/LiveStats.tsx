"use client";

import { useEffect, useState } from "react";
import { getGlobalStats, getWatchdogStats } from "@/lib/api";
import type { GlobalStats, WatchdogStats } from "@/lib/types";

interface Props {
  className?: string;
}

interface Combined {
  global: GlobalStats | null;
  watchdog: WatchdogStats | null;
}

export function LiveStats({ className }: Props) {
  const [data, setData] = useState<Combined>({ global: null, watchdog: null });
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const [g, w] = await Promise.all([
          getGlobalStats().catch(() => null),
          getWatchdogStats().catch(() => null),
        ]);
        if (!cancelled) {
          setData({ global: g, watchdog: w });
        }
      } catch (e) {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : "failed to load stats");
        }
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, []);

  const items: Array<{ label: string; value: string | number }> = [
    {
      label: "Contracts indexed",
      value: data.global ? formatNum(data.global.tracked_contracts) : "--",
    },
    {
      label: "Events tracked",
      value: data.global ? formatNum(data.global.total_events) : "--",
    },
    {
      label: "Invocations traced",
      value: data.global ? formatNum(data.global.total_invocations) : "--",
    },
    {
      label: "Contracts monitored",
      value: data.watchdog ? formatNum(data.watchdog.total_monitored) : "--",
    },
  ];

  return (
    <div className={className}>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {items.map((it) => (
          <div
            key={it.label}
            className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4 text-center"
          >
            <div className="text-2xl font-bold tabular-nums">{it.value}</div>
            <div className="mt-1 text-xs uppercase tracking-wider text-[var(--color-text-secondary)]">
              {it.label}
            </div>
          </div>
        ))}
      </div>
      {error && (
        <p className="mt-2 text-center text-xs text-[var(--color-text-secondary)]">
          Live stats unavailable. Start the API to see them here.
        </p>
      )}
    </div>
  );
}

function formatNum(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(1)}K`;
  return `${(n / 1_000_000).toFixed(1)}M`;
}
