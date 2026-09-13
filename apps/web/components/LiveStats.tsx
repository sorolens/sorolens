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

  useEffect(() => {
    let cancelled = false;
    async function load() {
      const [g, w] = await Promise.all([
        getGlobalStats().catch(() => null),
        getWatchdogStats().catch(() => null),
      ]);
      if (!cancelled) {
        setData({ global: g, watchdog: w });
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
      value: formatNum(data.global?.tracked_contracts ?? 0),
    },
    {
      label: "Events tracked",
      value: formatNum(data.global?.total_events ?? 0),
    },
    {
      label: "Invocations traced",
      value: formatNum(data.global?.total_invocations ?? 0),
    },
    {
      label: "Contracts monitored",
      value: formatNum(data.watchdog?.total_monitored ?? 0),
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
    </div>
  );
}

function formatNum(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(1)}K`;
  return `${(n / 1_000_000).toFixed(1)}M`;
}
