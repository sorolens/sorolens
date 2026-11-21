"use client";

import {
  Fragment,
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import Link from "next/link";
import { MonoId } from "@sorolens/ui";
import { listAllEvents } from "@/lib/api";
import { networkFilter, useNetwork } from "@/lib/network";
import { TableSkeleton } from "@/components/Skeleton";
import { EmptyState } from "@/components/EmptyState";
import {
  FIRST_PAGE,
  dateRangeError,
  dateRangeToBounds,
  formatEventData,
  popPage,
  previewEventData,
  pushPage,
  parseEventsUrlState,
  serializeEventsUrlState,
  type EventsUrlState,
  type EventTypeFilter,
} from "@/lib/eventsExplorer";
import { useQueryState } from "@/hooks/useQueryState";
import type { GlobalEvent } from "@/lib/types";
import { useLocale } from "@/lib/i18n";

const PAGE_SIZE = 25;

const EVENT_TYPES: { value: EventTypeFilter; label: string }[] = [
  { value: "", label: "All types" },
  { value: "contract", label: "Contract" },
  { value: "system", label: "System" },
  { value: "diagnostic", label: "Diagnostic" },
];

interface Filters {
  contractId: string;
  type: EventTypeFilter;
  from: string;
  to: string;
}

const NO_FILTERS: Filters = { contractId: "", type: "", from: "", to: "" };

const inputClass =
  "rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-1.5 text-xs text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]";

const buttonClass =
  "rounded-md border border-[var(--color-border)] px-3 py-1.5 text-xs font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] disabled:cursor-not-allowed disabled:opacity-40";

function EventRow({ event }: { event: GlobalEvent }) {
  const { formatNumber, formatDateTime } = useLocale();
  const [expanded, setExpanded] = useState(false);
  const data = event.value_decoded ?? event.value_xdr;

  return (
    <>
      <tr className="border-b border-[var(--color-border)] align-top transition-colors hover:bg-white/5">
        <td className="whitespace-nowrap px-4 py-3 font-mono text-xs">
          <MonoId value={event.contract_id} headChars={6} tailChars={6} />
        </td>
        <td className="whitespace-nowrap px-4 py-3 text-sm">
          <span className="rounded bg-white/5 px-2 py-0.5 text-xs">
            {event.type}
          </span>
        </td>
        <td className="px-4 py-3">
          <button
            type="button"
            onClick={() => setExpanded((v) => !v)}
            aria-expanded={expanded}
            className="flex max-w-xs items-center gap-2 text-left font-mono text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
          >
            <span aria-hidden>{expanded ? "▾" : "▸"}</span>
            <span className="truncate">{previewEventData(data)}</span>
          </button>
        </td>
        <td className="whitespace-nowrap px-4 py-3 text-right font-mono text-xs text-[var(--color-text-secondary)]">
          {formatNumber(event.ledger)}
        </td>
        <td
          className="whitespace-nowrap px-4 py-3 text-xs text-[var(--color-text-secondary)]"
          title={event.ledger_closed_at}
        >
          {formatDateTime(event.ledger_closed_at)}
        </td>
      </tr>
      {expanded && (
        <tr className="border-b border-[var(--color-border)] bg-white/[0.02]">
          <td colSpan={5} className="px-4 py-3">
            <div className="grid gap-3 md:grid-cols-2">
              <div>
                <p className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
                  Topics
                </p>
                <pre className="max-h-64 overflow-auto rounded bg-black/30 p-3 font-mono text-xs">
                  {formatEventData(event.topic_decoded ?? event.topic_xdr)}
                </pre>
              </div>
              <div>
                <p className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
                  Value
                </p>
                <pre className="max-h-64 overflow-auto rounded bg-black/30 p-3 font-mono text-xs">
                  {formatEventData(data)}
                </pre>
              </div>
            </div>
            <p className="mt-2 font-mono text-xs text-[var(--color-text-secondary)]">
              tx <MonoId value={event.tx_hash} headChars={10} tailChars={10} />
              {" · "}
              <Link
                href={`/contracts/${event.contract_id}`}
                className="text-[var(--color-accent)] hover:underline"
              >
                Open contract
              </Link>
            </p>
          </td>
        </tr>
      )}
    </>
  );
}

function EventsExplorerPage() {
  const { network } = useNetwork();
  const { t } = useLocale();

  // Applied filters and the current page live in the URL (issue #618), so
  // a filtered view survives reload and can be shared. The draft inputs are
  // local until the form is submitted.
  const [applied, setApplied] = useQueryState<EventsUrlState>({
    parse: parseEventsUrlState,
    serialize: serializeEventsUrlState,
  });

  const [draft, setDraft] = useState<Filters>({
    contractId: applied.contractId,
    type: applied.type,
    from: applied.from,
    to: applied.to,
  });
  const [events, setEvents] = useState<GlobalEvent[]>([]);
  const [nextCursor, setNextCursor] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  // The API paginates with forward-only opaque cursors, so "previous page"
  // is a client-side stack of the cursors each visited page was loaded
  // with. The URL holds the current cursor; the stack is session memory
  // and collapses to the current page whenever the cursor arrives from
  // the URL instead of our own pagination (reload, back, shared link).
  const [history, setHistory] = useState<string[]>([applied.cursor || ""]);
  useEffect(() => {
    setHistory((h) =>
      h[h.length - 1] === applied.cursor ? h : [applied.cursor || ""]
    );
  }, [applied.cursor]);

  // Keep the form fields in step with the applied filters when they change
  // through the URL (back/forward, shared link), without clobbering a
  // half-typed draft while the user pages through results.
  const appliedFilterKey = `${applied.contractId}\u0000${applied.type}\u0000${applied.from}\u0000${applied.to}`;
  const prevFilterKey = useRef(appliedFilterKey);
  useEffect(() => {
    if (prevFilterKey.current !== appliedFilterKey) {
      prevFilterKey.current = appliedFilterKey;
      setDraft({
        contractId: applied.contractId,
        type: applied.type,
        from: applied.from,
        to: applied.to,
      });
    }
  }, [appliedFilterKey, applied]);

  const filters = useMemo(
    () => ({
      contractId: applied.contractId,
      type: applied.type,
      from: applied.from,
      to: applied.to,
    }),
    [applied.contractId, applied.type, applied.from, applied.to]
  );
  const cursor = applied.cursor;

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    listAllEvents(
      {
        cursor: cursor || undefined,
        limit: PAGE_SIZE,
        contractId: filters.contractId || undefined,
        type: filters.type || undefined,
        network: networkFilter(network),
        ...dateRangeToBounds(filters.from, filters.to),
      },
      { signal: controller.signal }
    )
      .then((res) => {
        setEvents(res.events ?? []);
        setNextCursor(res.next_cursor ?? "");
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return;
        setEvents([]);
        setNextCursor("");
        setError(err instanceof Error ? err.message : "Failed to load events");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [cursor, filters, network, reloadKey]);

  // A network switch changes the result set, so start from the first page.
  const prevNetwork = useRef(network);
  useEffect(() => {
    if (prevNetwork.current === network) return;
    prevNetwork.current = network;
    setApplied(
      (prev) =>
        prev.cursor === "" && prev.page === 1
          ? prev
          : { ...prev, cursor: "", page: 1 },
      "replace"
    );
  }, [network, setApplied]);

  const rangeError = dateRangeError(draft.from, draft.to);
  const hasFilters = Boolean(
    filters.contractId || filters.type || filters.from || filters.to
  );

  const applyFilters = useCallback(
    (e: React.FormEvent) => {
      e.preventDefault();
      if (rangeError) return;
      const next: EventsUrlState = {
        contractId: draft.contractId.trim(),
        type: draft.type,
        from: draft.from,
        to: draft.to,
        cursor: "",
        page: 1,
      };
      setApplied(next);
      setHistory(FIRST_PAGE);
    },
    [draft, rangeError, setApplied]
  );

  const clearFilters = () => {
    setDraft(NO_FILTERS);
    setApplied((prev) => ({ ...prev, ...NO_FILTERS, cursor: "", page: 1 }));
    setHistory(FIRST_PAGE);
  };

  const goNextPage = () => {
    if (!nextCursor) return;
    setApplied((prev) => ({
      ...prev,
      cursor: nextCursor,
      page: prev.page + 1,
    }));
    setHistory((h) => pushPage(h, nextCursor));
  };

  const goPrevPage = () => {
    if (history.length <= 1) return;
    const prevCursor = history[history.length - 2];
    setApplied((prev) => ({
      ...prev,
      cursor: prevCursor,
      page: Math.max(1, prev.page - 1),
    }));
    setHistory((h) => popPage(h));
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">Events</h1>
          <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
            Every indexed event across all tracked contracts, newest first.
          </p>
        </div>
        <Link
          href="/events/live"
          className="text-sm text-[var(--color-accent)] hover:underline"
        >
          Live stream →
        </Link>
      </div>

      <form
        onSubmit={applyFilters}
        className="flex flex-wrap items-end gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4"
        aria-label="Event filters"
      >
        <label className="flex min-w-[14rem] flex-1 flex-col gap-1 text-xs text-[var(--color-text-secondary)]">
          Contract ID
          <input
            type="search"
            placeholder="C… (full ID or prefix)"
            value={draft.contractId}
            onChange={(e) => setDraft({ ...draft, contractId: e.target.value })}
            className={`${inputClass} font-mono`}
          />
        </label>
        <label className="flex flex-col gap-1 text-xs text-[var(--color-text-secondary)]">
          Event type
          <select
            value={draft.type}
            onChange={(e) =>
              setDraft({ ...draft, type: e.target.value as Filters["type"] })
            }
            className={inputClass}
          >
            {EVENT_TYPES.map((t) => (
              <option key={t.value} value={t.value}>
                {t.label}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1 text-xs text-[var(--color-text-secondary)]">
          From (UTC)
          <input
            type="date"
            value={draft.from}
            max={draft.to || undefined}
            onChange={(e) => setDraft({ ...draft, from: e.target.value })}
            className={inputClass}
          />
        </label>
        <label className="flex flex-col gap-1 text-xs text-[var(--color-text-secondary)]">
          To (UTC)
          <input
            type="date"
            value={draft.to}
            min={draft.from || undefined}
            onChange={(e) => setDraft({ ...draft, to: e.target.value })}
            className={inputClass}
          />
        </label>
        <div className="flex gap-2">
          <button
            type="submit"
            disabled={!!rangeError}
            className="rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-xs font-medium text-[var(--color-bg-page)] transition-opacity hover:opacity-90 disabled:opacity-40"
          >
            Apply
          </button>
          {(hasFilters ||
            JSON.stringify(draft) !== JSON.stringify(NO_FILTERS)) && (
            <button
              type="button"
              onClick={clearFilters}
              className={buttonClass}
            >
              Clear
            </button>
          )}
        </div>
        {rangeError && (
          <p role="alert" className="w-full text-xs text-red-400">
            {rangeError}
          </p>
        )}
      </form>

      {error ? (
        <EmptyState
          variant="error"
          title={t("events.error.title")}
          message={error}
          onRetry={() => setReloadKey((k) => k + 1)}
        />
      ) : loading && events.length === 0 ? (
        <TableSkeleton rows={8} />
      ) : events.length === 0 ? (
        <EmptyState
          variant={hasFilters ? "no-results" : "first-run"}
          title={
            hasFilters ? (
              t("events.empty.noResults.title")
            ) : (
              <>
                {t("events.empty.firstRun.title")}{" "}
                <Link
                  href="/contracts"
                  className="text-[var(--color-accent)] hover:underline"
                >
                  {t("empty.trackContract")}
                </Link>{" "}
                {t("events.empty.firstRun.suffix")}
              </>
            )
          }
          action={
            hasFilters
              ? { label: t("empty.clearFilters"), onClick: clearFilters }
              : undefined
          }
        />
      ) : (
        <div
          className={`overflow-hidden rounded-lg border border-[var(--color-border)] ${loading ? "opacity-60" : ""}`}
        >
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px] text-left">
              <thead className="border-b border-[var(--color-border)] text-xs uppercase tracking-wide text-[var(--color-text-secondary)]">
                <tr>
                  <th scope="col" className="px-4 py-3 font-medium">
                    Contract
                  </th>
                  <th scope="col" className="px-4 py-3 font-medium">
                    Type
                  </th>
                  <th scope="col" className="px-4 py-3 font-medium">
                    Data
                  </th>
                  <th scope="col" className="px-4 py-3 text-right font-medium">
                    Ledger
                  </th>
                  <th scope="col" className="px-4 py-3 font-medium">
                    Timestamp
                  </th>
                </tr>
              </thead>
              <tbody>
                {events.map((e) => (
                  <Fragment key={e.id}>
                    <EventRow event={e} />
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {!error && (events.length > 0 || applied.page > 1) && (
        <nav
          className="flex items-center justify-between"
          aria-label="Pagination"
        >
          <button
            type="button"
            onClick={goPrevPage}
            disabled={loading || history.length <= 1}
            className={buttonClass}
          >
            ← Previous
          </button>
          <span className="text-xs text-[var(--color-text-secondary)]">
            Page {applied.page}
          </span>
          <button
            type="button"
            onClick={goNextPage}
            disabled={loading || !nextCursor}
            className={buttonClass}
          >
            Next →
          </button>
        </nav>
      )}
    </div>
  );
}

export default function EventsPage() {
  // useSearchParams (inside useQueryState) requires a Suspense boundary for
  // static rendering.
  return (
    <Suspense fallback={<TableSkeleton rows={8} />}>
      <EventsExplorerPage />
    </Suspense>
  );
}
