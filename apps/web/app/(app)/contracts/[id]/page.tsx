"use client";

import { use, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { notFound, useRouter } from "next/navigation";
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
  ContractStats,
  StorageEntry,
  VolumePoint,
  ResourceTrendPoint,
  TimeWindow,
} from "@/lib/types";
import { getActivitySeries } from "@/lib/activitySeries";
import { ensureSession } from "@/lib/user";
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
import { ErrorState, errorMessage } from "@/components/ErrorState";
import { StoragePanel } from "@/components/StoragePanel";
import { ContractNotesPanel } from "@/components/ContractNotesPanel";
import { SnapshotPanel } from "@/components/SnapshotPanel";
import { HealthScoreCard } from "@/components/HealthScoreCard";
import { AddToGroup } from "@/components/AddToGroup";
import { VerifiedBadge } from "@/components/VerifiedBadge";
import { CallTracePanel } from "@/components/CallTracePanel";
import { ContractChangelogPanel } from "@/components/ContractChangelogPanel";
import { ContractForecastChart } from "@/components/ContractForecastChart";
import { useEventStream } from "@/hooks/useEventStream";

interface Props {
  params: Promise<{ id: string }>;
}

export default function ContractDetailPage({ params }: Props) {
  const { id } = use(params);
  return <ContractDetailContent id={id} />;
}

function ContractDetailContent({ id }: { id: string }) {
  const router = useRouter();
  const [contract, setContract] = useState<ContractDetail | null>(null);
  const [contractError, setContractError] = useState<string | null>(null);
  const [contractLoading, setContractLoading] = useState(true);
  // Bumped by each retry button to re-run that section's load effect.
  const [contractReload, setContractReload] = useState(0);
  const [statsReload, setStatsReload] = useState(0);
  const [seriesReload, setSeriesReload] = useState(0);
  const [eventsReload, setEventsReload] = useState(0);
  const [storageReload, setStorageReload] = useState(0);
  // A failed section shows an error with a retry instead of looking empty.
  const [statsError, setStatsError] = useState<string | null>(null);
  const [seriesError, setSeriesError] = useState<string | null>(null);
  const [eventsError, setEventsError] = useState<string | null>(null);
  const [storageError, setStorageError] = useState<string | null>(null);
  // Failed "load more" requests keep the rows already shown.
  const [eventsMoreError, setEventsMoreError] = useState<string | null>(null);
  const [storageMoreError, setStorageMoreError] = useState<string | null>(null);

  // User-defined tags are edited inline on this page.
  const [tags, setTags] = useState<string[]>([]);
  const [tagError, setTagError] = useState<string | null>(null);
  const [tagSaving, setTagSaving] = useState(false);

  // Pending destructive action awaiting confirmation.
  const [pendingRemoveTag, setPendingRemoveTag] = useState<string | null>(null);

  const [window, setWindow] = useState<TimeWindow>("7d");
  const [stats, setStats] = useState<ContractStats | null>(null);
  const [eventSeries, setEventSeries] = useState<VolumePoint[]>([]);
  const [invocationSeries, setInvocationSeries] = useState<VolumePoint[]>([]);
  const [seriesLoading, setSeriesLoading] = useState(true);

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
            // Render the route's not-found boundary so the response is a
            // real 404 rather than an in-page "not found" string.
            notFound();
            return;
          }
          // Anything but a 404 is an outage or server error, not a missing
          // contract: say so and let the user retry.
          setContractError(errorMessage(err, "The contract request failed."));
        }
      } finally {
        if (!cancelled) setContractLoading(false);
      }
    }

    load();
    return () => {
      cancelled = true;
    };
  }, [id, contractReload]);

  useEffect(() => {
    let cancelled = false;

    async function loadStats() {
      setStatsError(null);
      try {
        // The documented endpoint only aggregates 24h/7d/30d; "all" displays
        // totals, so the window counts are simply not shown for it.
        const data = await getContractStats(
          id,
          window === "all" ? "24h" : window
        );
        if (!cancelled) setStats(data);
      } catch (err) {
        if (!cancelled)
          setStatsError(errorMessage(err, "The stats request failed."));
      }
    }

    loadStats();
    return () => {
      cancelled = true;
    };
  }, [id, window, statsReload]);

  useEffect(() => {
    let cancelled = false;

    async function loadSeries() {
      setSeriesLoading(true);
      setSeriesError(null);
      try {
        const data = await getActivitySeries(id, window);
        if (!cancelled) {
          setEventSeries(data.events);
          setInvocationSeries(data.invocations);
        }
      } catch (err) {
        if (!cancelled)
          setSeriesError(errorMessage(err, "The activity request failed."));
      } finally {
        if (!cancelled) setSeriesLoading(false);
      }
    }

    loadSeries();
    return () => {
      cancelled = true;
    };
  }, [id, window, seriesReload]);

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
      setEventsError(null);
      setEventsMoreError(null);
      try {
        const data = await getContractEvents(id, {
          limit: 50,
          in_successful_call: showFailedOnly ? false : undefined,
        });
        if (!cancelled) {
          setEvents(data.events);
          setEventsCursor(data.next_cursor || null);
          setEventsHasMore(Boolean(data.next_cursor));
        }
      } catch (err) {
        if (!cancelled)
          setEventsError(errorMessage(err, "The events request failed."));
      } finally {
        if (!cancelled) setEventsLoading(false);
      }
    }

    loadEvents();
    return () => {
      cancelled = true;
    };
  }, [id, showFailedOnly, eventsReload]);

  useEffect(() => {
    let cancelled = false;

    async function loadStorage() {
      setStorageLoading(true);
      setStorageError(null);
      setStorageMoreError(null);
      try {
        const data = await getContractStorage(id, { limit: 100 });
        if (!cancelled) {
          setStorage(data.storage);
          setStorageCursor(data.next_cursor || null);
          setStorageHasMore(Boolean(data.next_cursor));
        }
      } catch (err) {
        if (!cancelled)
          setStorageError(errorMessage(err, "The storage request failed."));
      } finally {
        if (!cancelled) setStorageLoading(false);
      }
    }

    loadStorage();
    return () => {
      cancelled = true;
    };
  }, [id, storageReload]);

  const handleLoadMoreEvents = useCallback(async () => {
    if (!eventsCursor) return;
    setEventsMoreError(null);
    try {
      const data = await getContractEvents(id, {
        cursor: eventsCursor,
        limit: 50,
        in_successful_call: showFailedOnly ? false : undefined,
      });
      setEvents((prev) => [...prev, ...data.events]);
      setEventsCursor(data.next_cursor || null);
      setEventsHasMore(Boolean(data.next_cursor));
    } catch (err) {
      setEventsMoreError(errorMessage(err, "The next page failed to load."));
    }
  }, [id, eventsCursor, showFailedOnly]);

  const handleLoadMoreStorage = useCallback(async () => {
    if (!storageCursor) return;
    setStorageMoreError(null);
    try {
      const data = await getContractStorage(id, {
        cursor: storageCursor,
        limit: 100,
      });
      setStorage((prev) => [...prev, ...data.storage]);
      setStorageCursor(data.next_cursor || null);
      setStorageHasMore(Boolean(data.next_cursor));
    } catch (err) {
      setStorageMoreError(errorMessage(err, "The next page failed to load."));
    }
  }, [id, storageCursor]);

  const handleAddTag = useCallback(
    async (tag: string) => {
      setTagSaving(true);
      setTagError(null);
      try {
        const session = await ensureSession();
        const res = await addContractTag(id, tag, session.user_id);
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
        const session = await ensureSession();
        await removeContractTag(id, tag, session.user_id);
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

  const handleConfirmRemoveTag = useCallback(async () => {
    if (!pendingRemoveTag) return;
    const tag = pendingRemoveTag;
    setPendingRemoveTag(null);
    await handleRemoveTag(tag);
  }, [pendingRemoveTag, handleRemoveTag]);

  const handleCancelRemoveTag = useCallback(() => {
    setPendingRemoveTag(null);
  }, []);

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
      <div className="mx-auto max-w-xl">
        <ErrorState
          title="Couldn't load this contract."
          message={contractError}
          onRetry={() => {
            setContractLoading(true);
            setContractError(null);
            setContractReload((n) => n + 1);
          }}
        />
        <p className="mt-4 text-center text-sm text-[var(--color-text-secondary)]">
          Or return to the{" "}
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

  const windowLabel =
    window === "all" ? null : (stats?.window_duration ?? window);

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
          <AddToGroup contractId={id} />
          <VerifiedBadge contractId={id} />
          {/* Compare entry point (#607): pre-selects this contract in the
              shared /compare?ids= URL; the picker fills the remaining slots. */}
          <Link
            data-testid="compare-entry-link"
            href={`/compare?ids=${encodeURIComponent(id)}`}
            className="rounded-full border border-[var(--color-border)] px-2.5 py-0.5 text-xs font-medium text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-accent)] hover:text-[var(--color-text-primary)]"
          >
            Compare
          </Link>
        </div>
        <div className="mt-1 flex gap-4 text-xs text-[var(--color-text-secondary)]">
          <span>Network: {contract?.network}</span>
          {stats && (
            <span>
              Last synced ledger: {stats.last_synced_ledger.toLocaleString()}
            </span>
          )}
        </div>
        <div className="mt-3 max-w-lg">
          <TagInput
            tags={tags}
            onAdd={handleAddTag}
            onRemove={(tag) => setPendingRemoveTag(tag)}
            disabled={tagSaving}
            error={tagError}
            placeholder="Add a tag (prod, staging…)"
          />
        </div>
      </header>

      {statsError && (
        <ErrorState
          compact
          className="mb-4"
          title="Couldn't load contract stats."
          message={statsError}
          onRetry={() => setStatsReload((n) => n + 1)}
        />
      )}
      <section className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label="Events"
          value={stats?.event_count?.toLocaleString() ?? "-"}
          subtext={
            stats && windowLabel
              ? `${stats.window_event_count.toLocaleString()} in ${windowLabel}`
              : "Total indexed"
          }
        />
        <StatCard
          label="Invocations"
          value={stats?.invocation_count?.toLocaleString() ?? "-"}
          subtext={
            stats && windowLabel
              ? `${stats.window_invocation_count.toLocaleString()} in ${windowLabel}`
              : "Total transactions"
          }
        />
        <StatCard
          label="Storage Entries"
          value={stats?.storage_count?.toLocaleString() ?? "-"}
          subtext="Currently tracked"
        />
        <StatCard
          label="Last Synced Ledger"
          value={stats?.last_synced_ledger?.toLocaleString() ?? "-"}
          subtext="Indexer progress"
        />
      </section>

      <section className="mb-8">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-xl font-semibold">Activity</h2>
          <WindowSelector selected={window} onChange={setWindow} />
        </div>
        {seriesLoading ? (
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <ChartSkeleton />
            <ChartSkeleton />
          </div>
        ) : seriesError ? (
          <ErrorState
            title="Couldn't load activity charts."
            message={seriesError}
            onRetry={() => setSeriesReload((n) => n + 1)}
          />
        ) : (
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <div className="rounded-lg bg-[var(--color-bg-card)] p-4">
              <h3 className="mb-3 text-sm font-medium text-[var(--color-text-secondary)]">
                Event Volume
              </h3>
              <EventVolumeChart data={eventSeries} />
            </div>
            <div className="rounded-lg bg-[var(--color-bg-card)] p-4">
              <h3 className="mb-3 text-sm font-medium text-[var(--color-text-secondary)]">
                Invocations
              </h3>
              <InvocationChart data={invocationSeries} />
            </div>
          </div>
        )}
      </section>

      <ContractForecastChart contractId={id} />

      <section className="mb-8">
        <div className="mb-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <h2 className="text-xl font-semibold">Events</h2>
            {isStreamConnected && (
              <span className="inline-flex items-center gap-1.5 rounded-full bg-green-900/40 px-2.5 py-0.5 text-xs font-medium text-green-400">
                <span className="h-1.5 w-1.5 rounded-full bg-green-400 animate-pulse" />
                Live
              </span>
            )}
          </div>
          <button
            type="button"
            onClick={() => setShowFailedOnly(!showFailedOnly)}
            className={`rounded-full px-3 py-1 text-sm font-medium transition-colors ${
              showFailedOnly
                ? "bg-red-500/20 text-red-500 hover:bg-red-500/30"
                : "bg-[var(--color-bg-card)] text-[var(--color-text-secondary)] hover:bg-[var(--color-border)] hover:text-[var(--color-text-primary)]"
            }`}
          >
            Failed Only
          </button>
        </div>
        {eventsLoading ? (
          <TableSkeleton />
        ) : eventsError ? (
          <ErrorState
            title="Couldn't load events."
            message={eventsError}
            onRetry={() => setEventsReload((n) => n + 1)}
          />
        ) : (
          <>
            <EventsTable
              events={events}
              onLoadMore={handleLoadMoreEvents}
              hasMore={eventsHasMore}
            />
            {eventsMoreError && (
              <ErrorState
                compact
                className="mt-4"
                title="Couldn't load more events."
                message={eventsMoreError}
                onRetry={handleLoadMoreEvents}
              />
            )}
          </>
        )}
      </section>

      <section className="mb-8">
        <h2 className="mb-4 text-xl font-semibold">Call trace</h2>
        <CallTracePanel initialTxHash={events[0]?.tx_hash} />
      </section>

      <section className="mb-8">
        <h2 className="mb-4 text-xl font-semibold">Notes</h2>
        <ContractNotesPanel contractId={id} />
      </section>

      <section className="mb-8">
        <h2 className="mb-4 text-xl font-semibold">Storage TTL</h2>
        {storageLoading ? (
          <TableSkeleton />
        ) : storageError ? (
          <ErrorState
            title="Couldn't load storage entries."
            message={storageError}
            onRetry={() => setStorageReload((n) => n + 1)}
          />
        ) : (
          <>
            <StoragePanel
              entries={storage}
              currentLedger={stats?.last_synced_ledger ?? 0}
              onLoadMore={handleLoadMoreStorage}
              hasMore={storageHasMore}
            />
            {storageMoreError && (
              <ErrorState
                compact
                className="mt-4"
                title="Couldn't load more storage entries."
                message={storageMoreError}
                onRetry={handleLoadMoreStorage}
              />
            )}
          </>
        )}
      </section>

      <section className="mb-8" id="changelog">
        <h2 className="mb-4 text-xl font-semibold">Changelog</h2>
        <ContractChangelogPanel contractId={id} />
      </section>

      <section className="mb-8">
        <h2 className="mb-4 text-xl font-semibold">Snapshot / replay</h2>
        <SnapshotPanel
          contractId={id}
          currentLedger={stats?.last_synced_ledger ?? 0}
        />
      </section>

      <section className="mb-8">
        <HealthScoreCard contractId={id} />
      </section>

      <ConfirmDialog
        open={pendingRemoveTag !== null}
        title="Remove this tag?"
        description={
          pendingRemoveTag
            ? `The tag "${pendingRemoveTag}" will be removed from this contract.`
            : undefined
        }
        confirmLabel="Remove tag"
        destructive
        onConfirm={handleConfirmRemoveTag}
        onCancel={handleCancelRemoveTag}
      />
    </div>
  );
}

function SkeletonHeader() {
  return (
    <div role="status" aria-busy="true" aria-label="Loading contract details">
      <span className="sr-only">Loading contract details</span>
      <div aria-hidden="true" className="animate-pulse space-y-3">
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
    </div>
  );
}

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  description?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  destructive?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

/**
 * Shared, accessible confirmation dialog for destructive actions.
 *
 * Renders as a modal dialog with proper ARIA roles, traps focus on the
 * cancel button when opened, closes on Escape, and restores focus to the
 * previously focused element when dismissed.
 */
function ConfirmDialog({
  open,
  title,
  description,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  destructive = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const cancelRef = useRef<HTMLButtonElement>(null);
  const previouslyFocused = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!open) return;
    previouslyFocused.current =
      typeof document !== "undefined"
        ? (document.activeElement as HTMLElement | null)
        : null;
    // Move focus into the dialog once it is mounted.
    cancelRef.current?.focus();

    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.preventDefault();
        onCancel();
      }
    }

    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      previouslyFocused.current?.focus?.();
    };
  }, [open, onCancel]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onCancel}
    >
      <div
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-dialog-title"
        aria-describedby={description ? "confirm-dialog-description" : undefined}
        className="w-full max-w-md rounded-lg bg-[var(--color-bg-card)] p-6 shadow-xl"
        onClick={(event) => event.stopPropagation()}
      >
        <h2
          id="confirm-dialog-title"
          className="text-lg font-semibold text-[var(--color-text-primary)]"
        >
          {title}
        </h2>
        {description && (
          <p
            id="confirm-dialog-description"
            className="mt-2 text-sm text-[var(--color-text-secondary)]"
          >
            {description}
          </p>
        )}
        <div className="mt-6 flex justify-end gap-3">
          <button
            ref={cancelRef}
            type="button"
            onClick={onCancel}
            className="rounded-md bg-[var(--color-bg-card)] px-4 py-2 text-sm font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-border)] hover:text-[var(--color-text-primary)]"
          >
            {cancelLabel}
          </button>
          <button
            type="button"
            onClick={onConfirm}
            className={`rounded-md px-4 py-2 text-sm font-medium text-white ${
              destructive
                ? "bg-red-600 hover:bg-red-500"
                : "bg-[var(--color-accent)] hover:opacity-80"
            }`}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
