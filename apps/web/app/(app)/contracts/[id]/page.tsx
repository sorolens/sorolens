"use client";

import { use, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { notFound } from "next/navigation";
import { MonoId } from "@sorolens/ui";
import {
  getContract,
  getContractEvents,
  getContractStorage,
  getContractStats,
  getContractSnapshot,
  addContractTag,
  removeContractTag,
  ApiError,
} from "@/lib/api";
import type {
  ContractDetail,
  ContractEvent,
  StorageEntry,
  StatsResponse,
  ResourceTrendPoint,
  TimeWindow,
} from "@/lib/types";
import { getUserId } from "@/lib/user";
import { StatCard } from "@/components/StatCard";
import { TagInput } from "@/components/TagInput";
import {
  CardSkeleton,
  ChartSkeleton,
  TableSkeleton,
} from "@/components/Skeleton";
import { WindowSelector } from "@/components/WindowSelector";
import { EventVolumeChart } from "@/components/EventVolumeChart";
import { InvocationChart } from "@/components/InvocationChart";
import { ResourceTrendChart } from "@/components/ResourceTrendChart";
import { getResourceTrend } from "@/lib/resourceTrend";
import { EventsTable } from "@/components/EventsTable";
import { StoragePanel } from "@/components/StoragePanel";
import { ContractNotesPanel } from "@/components/ContractNotesPanel";
import { SnapshotPanel } from "@/components/SnapshotPanel";
import { HealthScoreCard } from "@/components/HealthScoreCard";
import { AddToGroup } from "@/components/AddToGroup";
import { VerifiedBadge } from "@/components/VerifiedBadge";
import { CallTracePanel } from "@/components/CallTracePanel";
import { ContractChangelogPanel } from "@/components/ContractChangelogPanel";
import { useEventStream } from "@/hooks/useEventStream";
import { ExtensionHandoff } from "@/components/ExtensionHandoff";

// Contract detail page: surfaces extension hand-off so users can open the
// current contract directly in the SoroLens browser extension.
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

  // User-defined tags are edited inline on this page.
  const [tags, setTags] = useState<string[]>([]);
  const [tagError, setTagError] = useState<string | null>(null);
  const [tagSaving, setTagSaving] = useState(false);

  const [window, setWindow] = useState<TimeWindow>("7d");
  const [stats, setStats] = useState<StatsResponse | null>(null);
  const [statsLoading, setStatsLoading] = useState(true);

  const [trend, setTrend] = useState<ResourceTrendPoint[]>([]);
  const [trendLoading, setTrendLoading] = useState(true);

  const [events, setEvents] = useState<ContractEvent[]>([]);
  const [eventsLoading, setEventsLoading] = useState(true);
  const [eventsCursor, setEventsCursor] = useState<string | null>(null);
  const [eventsHasMore, setEventsHasMore] = useState(false);
  const [showFailedOnly, setShowFailedOnly] = useState(false);

  const [storage, setStorage] = useState<StorageEntry[]>([]);
  const [storageLoading, setStorageLoading] = useState(true);
  const [storageCursor, setStorageCursor] = useState<string | null>(null);
  const [storageHasMore, setStorageHasMore] = useState(false);
  const [currentLedger, setCurrentLedger] = useState(0);

  const { isConnected: isStreamConnected } = useEventStream({
    contractId: id,
    onEvent: (newEvent) => {
      setEvents((prev) => {
        if (prev.some((e) => e.id === newEvent.id)) return prev;
        return [newEvent, ...prev];
      });
    },
  });

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const data = await getContract(id);
        if (!cancelled) {
          setContract(data);
          setTags(data.tags ?? []);
        }
      } catch (err) {
        if (!cancelled) {
          if (err instanceof ApiError && err.status === 404) {
            notFound();
          }
          // Backend not reachable. Surface a friendly not-found panel
          // rather than a red error, since the user cannot distinguish
          // "we do not have this contract" from "the API is down."
          setContractError("Contract not found");
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

    async function loadTrend() {
      setTrendLoading(true);
      try {
        const data = await getResourceTrend(id, 30);
        if (!cancelled) setTrend(data);
      } catch {
        // non-critical
      } finally {
        if (!cancelled) setTrendLoading(false);
      }
    }

    loadTrend();
    return () => {
      cancelled = true;
    };
  }, [id]);

  useEffect(() => {
    let cancelled = false;

    async function loadEvents() {
      setEventsLoading(true);
      try {
        const data = await getContractEvents(id, {
          limit: 50,
          in_successful_call: showFailedOnly ? false : undefined,
        });
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
  }, [id, showFailedOnly]);

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
      const data = await getContractEvents(id, {
        cursor: eventsCursor,
        limit: 50,
        in_successful_call: showFailedOnly ? false : undefined,
      });
      setEvents((prev) => [...prev, ...data.events]);
      setEventsCursor(data.cursor);
      setEventsHasMore(data.has_more);
    } catch {
      // non-critical
    }
  }, [id, eventsCursor, showFailedOnly]);

  const handleLoadMoreStorage = useCallback(async () => {
    if (!storageCursor) return;
    try {
      const data = await getContractStorage(id, {
        cursor: storageCursor,
        limit: 100,
      });
      setStorage((prev) => [...prev, ...data.entries]);
      setStorageCursor(data.cursor);
      setStorageHasMore(data.has_more);
    } catch {
      // non-critical
    }
  }, [id, storageCursor]);

  const handleAddTag = useCallback(
    async (tag: string) => {
      setTagSaving(true);
      setTagError(null);
      try {
        const res = await addContractTag(id, tag, getUserId());
        setTags(res.tags ?? []);
      } catch (err) {
        setTagError(
          err instanceof ApiError && err.status === 401
            ? "You need a contributor identity to edit tags."
            : "Failed to add tag."
        );
      } finally {
        setTagSaving(false);
      }
    },
    [id]
  );

  const handleRemoveTag = useCallback(
    async (tag: string) => {
      setTagSaving(true);
      setTagError(null);
      try {
        await removeContractTag(id, tag, getUserId());
        setTags((prev) => prev.filter((t) => t !== tag));
      } catch (err) {
        setTagError(
          err instanceof ApiError && err.status === 401
            ? "You need a contributor identity to edit tags."
            : "Failed to remove tag."
        );
      } finally {
        setTagSaving(false);
      }
    },
    [id]
  );

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
      <div className="mx-auto max-w-xl rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-8 py-16 text-center">
        <p className="text-lg font-medium text-[var(--color-text-primary)]">
          {contractError}
        </p>
        <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
          Check the contract ID and try again, or return to the{" "}
          <Link
            href="/contracts"
            className="text-[var(--color-accent)] underline underline-offset-2 hover:opacity-80"
          >
            contracts list
          </Link>
          .
        </p>
      </div>
    );
  }

  const eventVolumeData = stats?.event_volume ?? [];
  const invocationData = stats?.invocation_count ?? [];
  const contractStats = stats?.stats;

  return (
    <div>
      {/* Deep-link hand-off: passes the contract ID to the extension. */}
      <div className="mb-6">
        <ExtensionHandoff contractId={id} />
      </div>

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
          <AddToGroup contractId={id} />
          <VerifiedBadge contractId={id} />
        </div>
        {contract?.sync && (
          <div className="mt-1 flex gap-4 text-xs text-[var(--color-text-secondary)]">
            <span>Network: {contract.network}</span>
            <span>
              Last ledger: {contract.sync.last_ledger.toLocaleString()}
            </span>
            <span>
              Last sync: {new Date(contract.sync.last_run_at).toLocaleString()}
            </span>
          </div>
        )}
        <div className="mt-3 max-w-lg">
          <TagInput
            tags={tags}
            onAdd={handleAddTag}
            onRemove={handleRemoveTag}
            disabled={tagSaving}
            error={tagError}
            placeholder="Add a tag (prod, staging…)"
          />
        </div>
      </header>

      <section className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Events"
          value={contractStats?.event_count ?? 0}
          loading={statsLoading}
        />
        <StatCard
          label="Invocations"
          value={contractStats?.invocation_count ?? 0}
          loading={statsLoading}
        />
        <StatCard
          label="Failed"
          value={contractStats?.failed_count ?? 0}
          loading={statsLoading}
        />
        <StatCard
          label="Last ledger"
          value={contractStats?.last_ledger ?? 0}
          loading={statsLoading}
        />
      </section>

      <div className="mb-8 flex items-center justify-between">
        <h2 className="text-lg font-semibold">Activity</h2>
        <WindowSelector value={window} onChange={setWindow} />
      </div>

      <div className="mb-8 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <EventVolumeChart data={eventVolumeData} loading={statsLoading} />
        <InvocationChart data={invocationData} loading={statsLoading} />
      </div>

      <div className="mb-8">
        <ResourceTrendChart data={trend} loading={trendLoading} />
      </div>

      <div className="mb-8 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <HealthScoreCard contractId={id} />
        <SnapshotPanel contractId={id} />
      </div>

      <div className="mb-8">
        <CallTracePanel contractId={id} />
      </div>

      <div className="mb-8">
        <EventsTable
          events={events}
          loading={eventsLoading}
          hasMore={eventsHasMore}
          onloadMore={handleLoadMoreEvents}
          showFailedOnly={showFailedOnly}
          onShowFailedOnlyChange={setShowFailedOnly}
        />
      </div>

      <div className="mb-8">
        <StoragePanel
          entries={storage}
          loading={storageLoading}
          hasMore={storageHasMore}
          onLoadMore={handleLoadMoreStorage}
          currentLedger={currentLedger}
        />
      </div>

      <div className="mb-8 grid grid-cols-1 gap-4 lg:grid-cols-2">
        <ContractNotesPanel contractId={id} />
        <ContractChangelogPanel contractId={id} />
      </div>
    </div>
  );
}

function SkeletonHeader() {
  return (
    <div className="animate-pulse">
      <div className="mb-2 h-3 w-24 rounded bg-[var(--color-bg-card)]" />
      <div className="h-8 w-64 rounded bg-[var(--color-bg-card)]" />
    </div>
  );
}
