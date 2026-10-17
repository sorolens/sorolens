"use client";

import { useEffect, useState } from "react";
import { getGlobalStats, getWatchdogStats } from "@/lib/api";
import type { GlobalStats, WatchdogStats } from "@/lib/types";

interface Props {
  className?: string;
  initialData: Combined;
}

interface Combined {
  global: GlobalStats | null;
  watchdog: WatchdogStats | null;
}

export function LiveStatsClient({ className, initialData }: Props) {
  const [data, setData] = useState<Combined>(initialData);

  useEffect(() => {
    let cancelled = false;
    async function refresh() {
      const [global, watchdog] = await Promise.allSettled([
        getGlobalStats(),
        getWatchdogStats(),
      ]);
      if (cancelled) return;
      if (global.status === "rejected") {
        console.error("Unable to refresh global stats", global.reason);
      }
      if (watchdog.status === "rejected") {
        console.error("Unable to refresh watchdog stats", watchdog.reason);
      }
      setData((current) => ({
        global: global.status === "fulfilled" ? global.value : current.global,
        watchdog:
          watchdog.status === "fulfilled" ? watchdog.value : current.watchdog,
      }));
    }
    refresh();
    return () => {
      cancelled = true;
    };
  }, []);

  const items: Array<{ label: string; value: string }> = [
    {
      label: "Contracts indexed",
      value: data.global ? formatNum(data.global.tracked_contracts) : "—",
    },
    {
      label: "Events tracked",
      value: data.global ? formatNum(data.global.total_events) : "—",
    },
    {
      label: "Invocations traced",
      value: data.global ? formatNum(data.global.total_invocations) : "—",
    },
    {
      label: "Contracts monitored",
      value: data.watchdog ? formatNum(data.watchdog.total_monitored) : "—",
    },
  ];

  return (
    <div className={className}>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {items.map((item) => (
          <div
            key={item.label}
            className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4 text-center"
          >
            <div className="text-2xl font-bold tabular-nums">{item.value}</div>
            <div className="mt-1 text-xs uppercase tracking-wider text-[var(--color-text-secondary)]">
              {item.label}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function formatNum(value: number): string {
  if (value < 1000) return String(value);
  if (value < 1_000_000) return `${(value / 1000).toFixed(1)}K`;
  return `${(value / 1_000_000).toFixed(1)}M`;
}
