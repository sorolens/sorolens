"use client";

import { use, useCallback, useEffect, useState } from "react";
import { notFound } from "next/navigation";
import { MonoId } from "@sorolens/ui";
import {
  getContract,
  getContractEvents,
  getContractStorage,
  getContractStats,
  ApiError,
} from "@/lib/api";
import type {
  ContractDetail,
  ContractEvent,
  StorageEntry,
  StatsResponse,
  TimeWindow,
} from "@/lib/types";
import { StatCard } from "@/components/StatCard";
import { CardSkeleton, ChartSkeleton, TableSkeleton } from "@/components/Skeleton";
import { WindowSelector } from "@/components/WindowSelector";
import { EventVolumeChart } from "@/components/EventVolumeChart";
import { InvocationChart } from "@/components/InvocationChart";
import { EventsTable } from "@/components/EventsTable";
import { StoragePanel } from "@/components/StoragePanel";

interface Props {
  params: Promise<{ id: string }>;
}

export default function ContractDetailPage({ params }: Props) {
  const { id } = use(params);
  return <ContractDetailContent id={id} />;
}

function ContractDetailContent({ id }: { id: string }) {
  const [contract, setContract] = useState<ContractDetail | null>(null);
  const [contractError, setContractError] = useState<string | null>(null);
  const [contractLoading, setContractLoading] = useState(true);

  const [window, setWindow] = useState<TimeWindow>("7d");
  const [stats, setStats] = useState<StatsResponse | null>(null);
  const [statsLoading, setStatsLoading] = useState(true);

  const [events, setEvents] = useState<ContractEvent[]>([]);
  const [eventsLoading, setEventsLoading] = useState(true);
  const [eventsCursor, setEventsCursor] = useState<string | null>(null);
  const [eventsHasMore, setEventsHasMore] = useState(false);

  const [storage, setStorage] = useState<StorageEntry[]>([]);
  const [storageLoading, setStorageLoading] = useState(true);
  const [storageCursor, setStorageCursor] = useState<string | null>(null);
  const [storageHasMore, setStorageHasMore] = useState(false);
  const [currentLedger, setCurrentLedger] = useState(0);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const data = await getContract(id);
        if (!cancelled) setContract(data);
      } catch (err) {
        if (!cancelled) {
          if (err instanceof ApiError && err.status === 404) {
            notFound();
          }
          setContractError(err instanceof Error ? err.message : "Failed to load contract");
        }
      } finally {
        if (!cancelled) setContractLoading(false);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, [id]);

  useEffect(() => {
    let cancelled = false;

    async function loadStats() {
      setStatsLoading(true);
      try {
        const data = await getContractStats(id, window);
        if (!cancelled) setStats(data);
      } catch {
        // stats are non-critical
      } finally {
        if (!cancelled) setStatsLoading(false);
      }
    }

    loadStats();
    return () => {
      cancelled = true;
    };
  }, [id, window]);

  useEffect(() => {
    let cancelled = false;

    async function loadEvents() {
      setEventsLoading(true);
      try {
        const data = await getContractEvents(id, { limit: 50 });
        if (!cancelled) {
          setEvents(data.events);
          setEventsCursor(data.cursor);
          setEventsHasMore(data.has_more);
        }
      } catch {
        // non-critical
      } finally {
        if (!cancelled) setEventsLoading(false);
      }
    }

    loadEvents();
    return () => {
      cancelled = true;
    };
  }, [id]);

  useEffect(() => {
    let cancelled = false;

    async function loadStorage() {
      setStorageLoading(true);
      try {
        const data = await getContractStorage(id, { limit: 100 });
        if (!cancelled) {
          setStorage(data.entries);
          setStorageCursor(data.cursor);
          setStorageHasMore(data.has_more);
          setCurrentLedger(data.current_ledger);
        }
      } catch {
        // non-critical
      } finally {
        if (!cancelled) setStorageLoading(false);
      }
    }

    loadStorage();
    return () => {
      cancelled = true;
    };
  }, [id]);

  const handleLoadMoreEvents = useCallback(async () => {
    if (!eventsCursor) return;
    try {
      const data = await getContractEvents(id, { cursor: eventsCursor, limit: 50 });
      setEvents((prev) => [...prev, ...data.events]);
      setEventsCursor(data.cursor);
      setEventsHasMore(data.has_more);
    } catch {
      // non-critical
    }
  }, [id, eventsCursor]);

  const handleLoadMoreStorage = useCallback(async () => {
    if (!storageCursor) return;
    try {
      const data = await getContractStorage(id, { cursor: storageCursor, limit: 100 });
      setStorage((prev) => [...prev, ...data.entries]);
      setStorageCursor(data.cursor);
      setStorageHasMore(data.has_more);
    } catch {
      // non-critical
    }
  }, [id, storageCursor]);

  if (contractLoading) {
    return (
      <div>
        <header className="mb-8">
          <SkeletonHeader />
        </header>
        <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <CardSkeleton />
          <CardSkeleton />
          <CardSkeleton />
          <CardSkeleton />
        </div>
        <div className="mb-8 grid grid-cols-1 gap-4 lg:grid-cols-2">
          <ChartSkeleton />
          <ChartSkeleton />
        </div>
        <TableSkeleton />
      </div>
    );
  }

  if (contractError) {
    return (
      <div className="flex flex-col items-center justify-center py-24">
        <p className="mb-2 text-lg text-[var(--color-danger)]">{contractError}</p>
      </div>
    );
  }

  const eventVolumeData = stats?.event_volume ?? [];
  const invocationData = stats?.invocation_count ?? [];
  const contractStats = stats?.stats;

  return (
    <div>
      <header className="mb-8">
        <div className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
          Contract
        </div>
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-semibold">
            <MonoId value={id} headChars={8} tailChars={8} />
          </h1>
          {contract?.label && (
            <span className="text-lg text-[var(--color-text-secondary)]">
              {contract.label}
            </span>
          )}
          <span
            className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${
              contract?.status === "active"
                ? "bg-green-900/40 text-green-400"
                : contract?.status === "backfilling"
                  ? "bg-blue-900/40 text-blue-400"
                  : contract?.status === "error"
                    ? "bg-red-900/40 text-red-400"
                    : "bg-yellow-900/40 text-yellow-400"
            }`}
          >
            {contract?.status}
          </span>
        </div>
        {contract?.sync && (
          <div className="mt-1 flex gap-4 text-xs text-[var(--color-text-secondary)]">
            <span>Network: {contract.network}</span>
            <span>
              Last ledger: {contract.sync.last_ledger.toLocaleString()}
            </span>
            <span>
              Last sync:{" "}
              {new Date(contract.sync.last_run_at).toLocaleString()}
            </span>
          </div>
        )}
      </header>

      <section className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Events"
          value={contractStats?.total_events?.toLocaleString() ?? "-"}
          subtext="Total indexed"
        />
        <StatCard
          label="Invocations"
          value={contractStats?.total_invocations?.toLocaleString() ?? "-"}
          subtext="Total transactions"
        />
        <StatCard
          label="Storage Entries"
          value={contractStats?.storage_entry_count?.toLocaleString() ?? "-"}
          subtext="Currently tracked"
        />
        <StatCard
          label="Expiring Soon"
          value={contractStats?.expiring_entry_count?.toLocaleString() ?? "-"}
          subtext="Within 7 days"
          variant={
            (contractStats?.expiring_entry_count ?? 0) > 10
              ? "danger"
              : (contractStats?.expiring_entry_count ?? 0) > 0
                ? "warning"
                : "default"
          }
        />
      </section>

      <section className="mb-8">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-xl font-semibold">Activity</h2>
          <WindowSelector selected={window} onChange={setWindow} />
        </div>
        {statsLoading ? (
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <ChartSkeleton />
            <ChartSkeleton />
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <div className="rounded-lg bg-[var(--color-bg-card)] p-4">
              <h3 className="mb-3 text-sm font-medium text-[var(--color-text-secondary)]">
                Event Volume
              </h3>
              <EventVolumeChart data={eventVolumeData} />
            </div>
            <div className="rounded-lg bg-[var(--color-bg-card)] p-4">
              <h3 className="mb-3 text-sm font-medium text-[var(--color-text-secondary)]">
                Invocations
              </h3>
              <InvocationChart data={invocationData} />
            </div>
          </div>
        )}
      </section>

      <section className="mb-8">
        <h2 className="mb-4 text-xl font-semibold">Events</h2>
        {eventsLoading ? (
          <TableSkeleton />
        ) : (
          <EventsTable
            events={events}
            onLoadMore={handleLoadMoreEvents}
            hasMore={eventsHasMore}
          />
        )}
      </section>

      <section className="mb-8">
        <h2 className="mb-4 text-xl font-semibold">Storage TTL</h2>
        {storageLoading ? (
          <TableSkeleton />
        ) : (
          <StoragePanel
            entries={storage}
            currentLedger={currentLedger}
            onLoadMore={handleLoadMoreStorage}
            hasMore={storageHasMore}
          />
        )}
      </section>
    </div>
  );
}

function SkeletonHeader() {
  return (
    <div className="animate-pulse space-y-3">
      <div className="h-3 w-16 rounded bg-[var(--color-border)]" />
      <div className="flex items-center gap-3">
        <div className="h-8 w-48 rounded bg-[var(--color-border)]" />
        <div className="h-5 w-16 rounded-full bg-[var(--color-border)]" />
      </div>
      <div className="flex gap-4">
        <div className="h-3 w-32 rounded bg-[var(--color-border)]" />
        <div className="h-3 w-28 rounded bg-[var(--color-border)]" />
        <div className="h-3 w-36 rounded bg-[var(--color-border)]" />
      </div>
    </div>
  );
}
