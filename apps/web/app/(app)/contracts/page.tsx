"use client";

import {
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type { FormEvent, ReactNode } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { DataTable, Toast } from "@sorolens/ui";
import type { Column } from "@sorolens/ui";
import { LabelledId } from "@/components/LabelledId";
import { ApiError, batchContracts, listContracts } from "@/lib/api";
import type { BatchContractsAction } from "@/lib/types";
import { networkFilter, useNetwork } from "@/lib/network";
import { contractRowKey, isPendingRow } from "@/lib/optimisticTrack";
import type { ContractRow } from "@/lib/optimisticTrack";
import { getUserId } from "@/lib/user";
import { TableSkeleton } from "@/components/Skeleton";
import ImportContractsCsv from "@/components/ImportContractsCsv";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 20;

// Sort is stored in the URL (?sort=&dir=) so it is shareable (#175). Column
// keys must match the backend's whitelist (id, label, network, status,
// added_at).
const SORT_PARAM = "sort";
const DIR_PARAM = "dir";
const DEFAULT_SORT_COLUMN = "added_at";
const DEFAULT_SORT_DIRECTION = "desc" as const;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function StatusBadge({ status }: { status: string }) {
  const colorMap: Record<string, string> = {
    active: "bg-green-900/40 text-green-400",
    backfilling: "bg-blue-900/40 text-blue-400",
    error: "bg-red-900/40 text-red-400",
    inactive: "bg-yellow-900/40 text-yellow-400",
  };
  const cls = colorMap[status] ?? "bg-yellow-900/40 text-yellow-400";
  return (
    <span
      className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${cls}`}
    >
      {status}
    </span>
  );
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function formatRelativeTime(iso: string, now = Date.now()) {
  const diffSeconds = Math.max(
    0,
    Math.floor((now - new Date(iso).getTime()) / 1000)
  );
  if (diffSeconds < 60) return "just now";
  const minutes = Math.floor(diffSeconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function RelativeTime({ iso }: { iso: string | null }) {
  const [, setNow] = useState(() => Date.now());

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 30_000);
    return () => window.clearInterval(timer);
  }, []);

  if (!iso) {
    return (
      <span className="text-[var(--color-text-secondary)]">No activity</span>
    );
  }

  return <span>{formatRelativeTime(iso)}</span>;
}

// ---------------------------------------------------------------------------
// Bulk action modals (#176)
// ---------------------------------------------------------------------------

/** Shared modal chrome: backdrop click and Escape both close the dialog. */
function ModalShell({
  titleId,
  title,
  onClose,
  children,
}: {
  titleId: string;
  title: string;
  onClose: () => void;
  children: ReactNode;
}) {
  const backdropRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [onClose]);

  return (
    <div
      ref={backdropRef}
      onClick={(e) => {
        if (e.target === backdropRef.current) onClose();
      }}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      aria-modal="true"
      role="dialog"
      aria-labelledby={titleId}
    >
      <div className="w-full max-w-md rounded-xl bg-[var(--color-bg-card)] p-6 shadow-2xl border border-[var(--color-border)]">
        <div className="mb-5 flex items-center justify-between">
          <h2
            id={titleId}
            className="text-lg font-semibold text-[var(--color-text-primary)]"
          >
            {title}
          </h2>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors"
            aria-label="Close modal"
          >
            ✕
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

/** Confirmation for the destructive "untrack" bulk action. */
function UntrackConfirmModal({
  count,
  pending,
  onClose,
  onConfirm,
}: {
  count: number;
  pending: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  return (
    <ModalShell
      titleId="untrack-modal-title"
      title="Untrack contracts?"
      onClose={onClose}
    >
      <p className="text-sm text-[var(--color-text-secondary)]">
        This permanently removes{" "}
        <span className="font-medium text-[var(--color-text-primary)]">
          {count} {count === 1 ? "contract" : "contracts"}
        </span>{" "}
        and all of their indexed events, invocations and storage snapshots. This
        cannot be undone.
      </p>
      <div className="mt-6 flex gap-3">
        <button
          type="button"
          onClick={onClose}
          className="flex-1 rounded-lg border border-[var(--color-border)] px-4 py-2.5 text-sm font-medium text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors"
        >
          Cancel
        </button>
        <button
          id="untrack-confirm-btn"
          type="button"
          onClick={onConfirm}
          disabled={pending}
          className="flex-1 rounded-lg bg-red-600 px-4 py-2.5 text-sm font-semibold text-white transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {pending ? "Untracking…" : `Untrack ${count}`}
        </button>
      </div>
    </ModalShell>
  );
}

/** Collects the tag for the bulk "tag" action. */
function TagModal({
  count,
  pending,
  onClose,
  onSubmit,
}: {
  count: number;
  pending: boolean;
  onClose: () => void;
  onSubmit: (label: string) => void;
}) {
  const [label, setLabel] = useState("");
  const trimmed = label.trim();

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (!trimmed) return;
    onSubmit(trimmed);
  };

  return (
    <ModalShell titleId="tag-modal-title" title="Add tag" onClose={onClose}>
      <form onSubmit={handleSubmit} className="space-y-4">
        <p className="text-sm text-[var(--color-text-secondary)]">
          Applies one tag to {count} selected{" "}
          {count === 1 ? "contract" : "contracts"}, replacing any existing
          alias.
        </p>
        <div>
          <label
            htmlFor="tag-input"
            className="mb-1.5 block text-sm font-medium text-[var(--color-text-secondary)]"
          >
            Tag
          </label>
          <input
            id="tag-input"
            type="text"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder="payments"
            className="w-full rounded-lg border border-[var(--color-border)] bg-black/30 px-3 py-2.5 text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
        </div>
        <div className="flex gap-3 pt-1">
          <button
            type="button"
            onClick={onClose}
            className="flex-1 rounded-lg border border-[var(--color-border)] px-4 py-2.5 text-sm font-medium text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors"
          >
            Cancel
          </button>
          <button
            id="tag-submit-btn"
            type="submit"
            disabled={!trimmed || pending}
            className="flex-1 rounded-lg bg-[var(--color-accent)] px-4 py-2.5 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity disabled:opacity-50 hover:opacity-90"
          >
            {pending ? "Applying…" : "Apply tag"}
          </button>
        </div>
      </form>
    </ModalShell>
  );
}

// ---------------------------------------------------------------------------
// Contracts Table Columns
// ---------------------------------------------------------------------------

const COLUMNS: Column<ContractRow>[] = [
  {
    key: "id",
    header: "Contract ID",
    sortable: true,
    accessor: (c) => (
      <span
        className={`font-mono text-xs ${isPendingRow(c) ? "opacity-60" : ""}`}
      >
        <LabelledId value={c.id} knownLabel={c.label} />
      </span>
    ),
  },
  {
    key: "network",
    header: "Network",
    sortable: true,
    accessor: (c) => (
      <span className="text-xs text-[var(--color-text-secondary)]">
        {c.network}
      </span>
    ),
  },
  {
    key: "status",
    header: "Status",
    sortable: true,
    accessor: (c) => <StatusBadge status={c.status} />,
  },
  {
    key: "label",
    header: "Label",
    sortable: true,
    accessor: (c) => (
      <span className="text-xs text-[var(--color-text-secondary)]">
        {c.label}
      </span>
    ),
  },
  {
    key: "added_at",
    header: "Added",
    sortable: true,
    accessor: (c) => (
      <span className="text-xs text-[var(--color-text-secondary)]">
        {formatDate(c.added_at)}
      </span>
    ),
  },

  {
    key: "last_activity_at",
    header: "Last activity",
    sortable: true,
    accessor: (c) => (
      <span className="text-xs text-[var(--color-text-secondary)]">
        <RelativeTime iso={c.last_activity_at} />
      </span>
    ),
  },
];

/**
 * The base columns plus a Tags column whose chips feed the server-side tag
 * filter. Built per render rather than declared as a constant because the chips
 * need the click handler; the handler only calls `setTagFilter`, which is
 * stable, so `useMemo(..., [])` around the result stays correct.
 */
function makeColumns(onTagClick: (tag: string) => void): Column<ContractRow>[] {
  const tagsColumn: Column<ContractRow> = {
    key: "tags",
    header: "Tags",
    accessor: (c) =>
      c.tags && c.tags.length > 0 ? (
        <span className="flex flex-wrap gap-1">
          {c.tags.map((tag) => (
            <button
              key={tag}
              type="button"
              onClick={() => onTagClick(tag)}
              className="rounded-full bg-[var(--color-bg-card)] px-2 py-0.5 text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
            >
              {tag}
            </button>
          ))}
        </span>
      ) : (
        <span className="text-xs text-[var(--color-text-secondary)]">—</span>
      ),
  };

  // Sits after Status: the tags read as part of how a contract is identified,
  // before the date columns.
  const statusIndex = COLUMNS.findIndex((column) => column.key === "status");
  return [
    ...COLUMNS.slice(0, statusIndex + 1),
    tagsColumn,
    ...COLUMNS.slice(statusIndex + 1),
  ];
}

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

// useSearchParams must sit behind a Suspense boundary during static
// prerendering, so the page is split into a Suspense wrapper and the
// component that actually reads the URL.
export default function ContractsPage() {
  return (
    <Suspense fallback={null}>
      <ContractsPageInner />
    </Suspense>
  );
}

function ContractsPageInner() {
  // Selected network from the header selector.
  const { network } = useNetwork();

  // Sort state is derived from the URL and written back on change, so a
  // sorted view is shareable via its query string (#175).
  const router = useRouter();
  const searchParams = useSearchParams();
  const [sortColumn, setSortColumn] = useState<string>(
    () => searchParams?.get(SORT_PARAM) ?? DEFAULT_SORT_COLUMN
  );
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">(() =>
    searchParams?.get(DIR_PARAM) === "asc" ? "asc" : DEFAULT_SORT_DIRECTION
  );

  // Data state
  const [contracts, setContracts] = useState<ContractRow[]>([]);
  const [loading, setLoading] = useState(true);

  // Pagination state: stack of cursors, index 0 = first page
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const [cursorIndex, setCursorIndex] = useState(0);
  const [hasMore, setHasMore] = useState(false);

  // Search state
  const [search, setSearch] = useState("");

  // Tag filter state (server-side, combined with the network filter)
  const [tagFilter, setTagFilter] = useState("");

  // CSV import state
  const [showImport, setShowImport] = useState(false);

  // Bulk selection state (#176): ids of the currently checked contracts.
  // Pending (optimistic) rows are not selectable.
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [showUntrackModal, setShowUntrackModal] = useState(false);
  const [showTagModal, setShowTagModal] = useState(false);
  const [bulkPending, setBulkPending] = useState(false);

  // Track state: one request in flight at a time, errors surface as a toast.
  const [trackPending, setTrackPending] = useState(false);
  const [toast, setToast] = useState<{
    id: number;
    message: string;
    variant: "info" | "success" | "error";
  } | null>(null);
  const toastSeq = useRef(0);
  const dismissToast = useCallback(() => setToast(null), []);

  // ---------------------------------------------------------------------------
  // Data fetching
  // ---------------------------------------------------------------------------

  // Only the most recent load() may write to state, so a slow response can't
  // overwrite a newer page, the optimistic list, or a restored snapshot.
  const loadSeq = useRef(0);

  const load = useCallback(
    async (cursor: string | null) => {
      const seq = ++loadSeq.current;
      setLoading(true);
      try {
        const data = await listContracts({
          cursor: cursor ?? undefined,
          limit: PAGE_SIZE,
          network: networkFilter(network),
          tag: tagFilter || undefined,
          sort: sortColumn,
          dir: sortDirection,
        });
        if (seq !== loadSeq.current) return;
        setContracts(data.contracts ?? []);
        setHasMore(data.has_more ?? false);
      } catch {
        if (seq !== loadSeq.current) return;
        // Backend not reachable yet. Fall through to the empty state so the
        // page still reads as "waiting for data" instead of "broken".
        setContracts([]);
        setHasMore(false);
      } finally {
        if (seq === loadSeq.current) setLoading(false);
      }
    },
    [network, tagFilter, sortColumn, sortDirection]
  );

  useEffect(() => {
    load(cursors[cursorIndex]);
  }, [load, cursors, cursorIndex]);

  // Reset to the first page when the network or tag filter changes. The ref
  // guard keeps this from firing an extra fetch on mount.
  const prevNetwork = useRef(network);
  const prevTag = useRef(tagFilter);
  useEffect(() => {
    const networkChanged = prevNetwork.current !== network;
    const tagChanged = prevTag.current !== tagFilter;
    if (networkChanged || tagChanged) {
      prevNetwork.current = network;
      prevTag.current = tagFilter;
      setCursors([null]);
      setCursorIndex(0);
    }
  }, [network, tagFilter]);

  // Reset to the first page when the sort changes, because an in-flight
  // cursor was produced in the previous order and no longer points at the
  // next page under the new sort.
  const prevSort = useRef(`${sortColumn}:${sortDirection}`);
  useEffect(() => {
    const key = `${sortColumn}:${sortDirection}`;
    if (prevSort.current !== key) {
      prevSort.current = key;
      setCursors([null]);
      setCursorIndex(0);
    }
  }, [sortColumn, sortDirection]);

  // Mirror the active sort into the URL so the view is shareable. The guard
  // only writes when the URL differs, so visiting /contracts without sort
  // params does not rewrite the URL on mount.
  useEffect(() => {
    const params = new URLSearchParams(searchParams?.toString() ?? "");
    const current = params.get(SORT_PARAM) ?? DEFAULT_SORT_COLUMN;
    const currentDir =
      params.get(DIR_PARAM) === "asc" ? "asc" : DEFAULT_SORT_DIRECTION;
    if (current === sortColumn && currentDir === sortDirection) return;
    params.set(SORT_PARAM, sortColumn);
    params.set(DIR_PARAM, sortDirection);
    router.replace(`?${params.toString()}`);
  }, [sortColumn, sortDirection, router, searchParams]);

  // ---------------------------------------------------------------------------
  // Pagination handlers
  // ---------------------------------------------------------------------------

  const handleNext = () => {
    if (!hasMore || contracts.length === 0) return;
    const nextCursor = contracts[contracts.length - 1].id;
    const newCursors = [...cursors.slice(0, cursorIndex + 1), nextCursor];
    setCursors(newCursors);
    setCursorIndex(cursorIndex + 1);
  };

  const handlePrev = () => {
    if (cursorIndex === 0) return;
    setCursorIndex(cursorIndex - 1);
  };

  // ---------------------------------------------------------------------------
  // Sort handler
  // ---------------------------------------------------------------------------

  const handleSort = (col: string) => {
    if (col === sortColumn) {
      setSortDirection((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortColumn(col);
      setSortDirection("asc");
    }
  };

  // ---------------------------------------------------------------------------
  // Bulk selection + actions (#176)
  // ---------------------------------------------------------------------------

  const toggleRow = useCallback((id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const toggleAll = useCallback((ids: string[]) => {
    setSelected((prev) => {
      const next = new Set(prev);
      const allSelected = ids.length > 0 && ids.every((id) => next.has(id));
      for (const id of ids) {
        if (allSelected) next.delete(id);
        else next.add(id);
      }
      return next;
    });
  }, []);

  const clearSelection = useCallback(() => setSelected(new Set()), []);

  // runBulkAction calls the batch endpoint, then refreshes the current page and
  // clears the selection. Errors surface as a toast rather than being swallowed.
  const runBulkAction = async (
    action: BatchContractsAction,
    label?: string
  ) => {
    const ids = Array.from(selected);
    if (ids.length === 0) return;
    setBulkPending(true);
    try {
      const res = await batchContracts(
        { ids, action, args: label ? { label } : undefined },
        getUserId()
      );
      const noun = res.affected === 1 ? "contract" : "contracts";
      setToast({
        id: ++toastSeq.current,
        variant: "success",
        message:
          action === "untrack"
            ? `Untracked ${res.affected} ${noun}.`
            : `Tagged ${res.affected} ${noun} as “${label}”.`,
      });
      clearSelection();
      setShowUntrackModal(false);
      setShowTagModal(false);
      // The list changed, so reload the page the user is looking at.
      load(cursors[cursorIndex]);
    } catch (err) {
      const detail = err instanceof ApiError ? err.message : "";
      const verb = action === "untrack" ? "untrack" : "tag";
      setToast({
        id: ++toastSeq.current,
        variant: "error",
        message: detail
          ? `Couldn't ${verb} contracts: ${detail}`
          : `Couldn't ${verb} contracts. Please try again.`,
      });
    } finally {
      setBulkPending(false);
    }
  };

  // ---------------------------------------------------------------------------
  // Derived: filtered + sorted data (client-side for current page)
  // ---------------------------------------------------------------------------

  const filtered = contracts.filter((c) => {
    if (!search) return true;
    const q = search.toLowerCase();
    return (
      c.id.toLowerCase().includes(q) ||
      (c.label ?? "").toLowerCase().includes(q)
    );
  });

  const sorted = [...filtered].sort((a, b) => {
    // Keep a just-submitted contract at the top whatever the sort column.
    if (isPendingRow(a) !== isPendingRow(b)) return isPendingRow(a) ? -1 : 1;
    const av = (a as unknown as Record<string, unknown>)[sortColumn];
    const bv = (b as unknown as Record<string, unknown>)[sortColumn];
    const cmp = String(av ?? "").localeCompare(String(bv ?? ""));
    return sortDirection === "asc" ? cmp : -cmp;
  });

  const isFirstPage = cursorIndex === 0;
  const isLastPage = !hasMore;

  // ---------------------------------------------------------------------------
  // Track success: refresh first page
  // ---------------------------------------------------------------------------

  const handleTrackSuccess = () => {
    setCursors([null]);
    setCursorIndex(0);
  };

  // Selectable ids on the current page: pending (optimistic) rows are skipped.
  const pageIds = sorted.filter((c) => !isPendingRow(c)).map((c) => c.id);
  const allSelected =
    pageIds.length > 0 && pageIds.every((id) => selected.has(id));

  // Columns depend on the tag-click handler so a tag chip can set the filter,
  // and lead with a selection column whose header checkbox toggles every
  // selectable row on the current page.
  const columns = useMemo(() => {
    const selectColumn: Column<ContractRow> = {
      key: "select",
      header: (
        <input
          type="checkbox"
          data-testid="select-all"
          aria-label="Select all contracts on this page"
          checked={allSelected}
          onChange={() => toggleAll(pageIds)}
        />
      ),
      accessor: (c) => (
        <input
          type="checkbox"
          data-testid={`select-${c.id}`}
          aria-label={`Select contract ${c.id}`}
          checked={selected.has(c.id)}
          disabled={isPendingRow(c)}
          // Keep the checkbox from triggering the row's navigate-on-click.
          onClick={(e) => e.stopPropagation()}
          onChange={() => toggleRow(c.id)}
        />
      ),
    };
    return [selectColumn, ...makeColumns((tag) => setTagFilter(tag))];
  }, [allSelected, pageIds, selected, toggleAll, toggleRow]);

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <>
      {showImport && (
        <ImportContractsCsv
          network={network}
          userId={getUserId()}
          onClose={() => setShowImport(false)}
          onImported={handleTrackSuccess}
        />
      )}

      {showUntrackModal && (
        <UntrackConfirmModal
          count={selected.size}
          pending={bulkPending}
          onClose={() => setShowUntrackModal(false)}
          onConfirm={() => runBulkAction("untrack")}
        />
      )}

      {showTagModal && (
        <TagModal
          count={selected.size}
          pending={bulkPending}
          onClose={() => setShowTagModal(false)}
          onSubmit={(label) => runBulkAction("tag", label)}
        />
      )}

      {toast && (
        <Toast
          key={toast.id}
          message={toast.message}
          variant={toast.variant}
          onDismiss={dismissToast}
        />
      )}

      <div>
        {/* Header */}
        <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">
            Contracts
          </h1>

          <div className="flex gap-2">
            <button
              id="import-csv-btn"
              type="button"
              onClick={() => setShowImport(true)}
              disabled={trackPending}
              className="inline-flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-4 py-2 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
            >
              Import CSV
            </button>
            {/* The tracking wizard (issue #140) replaced the old single-field
                modal, so this is a route, not a dialog trigger. */}
            <Link
              id="track-contract-btn"
              href="/contracts/new"
              className="inline-flex items-center gap-2 rounded-lg bg-[var(--color-accent)] px-4 py-2 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90"
            >
              <span aria-hidden="true">+</span>
              Track contract
            </Link>
          </div>
        </div>

        {/* Filters: free-text search and server-side tag filter */}
        <div className="mb-4 flex flex-col gap-3 sm:flex-row">
          <input
            id="contracts-search"
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by alias or contract ID…"
            aria-label="Search contracts"
            className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-2.5 text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
          <input
            id="contracts-tag-filter"
            type="search"
            value={tagFilter}
            onChange={(e) => setTagFilter(e.target.value)}
            placeholder="Filter by tag…"
            aria-label="Filter contracts by tag"
            className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-2.5 text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none sm:max-w-xs"
          />
        </div>

        {/* Bulk actions toolbar: shown only while a selection exists (#176) */}
        {selected.size > 0 && (
          <div
            id="bulk-toolbar"
            className="mb-4 flex flex-wrap items-center gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-3"
          >
            <span className="text-sm text-[var(--color-text-secondary)]">
              {selected.size} selected
            </span>
            <div className="flex flex-1 flex-wrap items-center justify-end gap-2">
              <button
                id="bulk-tag-btn"
                type="button"
                onClick={() => setShowTagModal(true)}
                disabled={bulkPending}
                className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
              >
                Add tag
              </button>
              <button
                id="bulk-untrack-btn"
                type="button"
                onClick={() => setShowUntrackModal(true)}
                disabled={bulkPending}
                className="rounded-lg border border-red-500/40 bg-red-600/10 px-3 py-1.5 text-sm font-medium text-red-400 transition-colors hover:bg-red-600/20 disabled:cursor-not-allowed disabled:opacity-50"
              >
                Untrack {selected.size}{" "}
                {selected.size === 1 ? "contract" : "contracts"}
              </button>
              <button
                id="bulk-clear-btn"
                type="button"
                onClick={clearSelection}
                disabled={bulkPending}
                className="rounded-lg px-3 py-1.5 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] disabled:opacity-50"
              >
                Clear
              </button>
            </div>
          </div>
        )}

        {/* Loading skeleton */}
        {loading && <TableSkeleton rows={PAGE_SIZE} />}

        {/* Empty state: also shown when the API is unreachable */}
        {!loading && sorted.length === 0 && (
          <div className="rounded-lg bg-[var(--color-bg-card)] px-8 py-16 text-center border border-[var(--color-border)]">
            <p className="text-lg font-medium text-[var(--color-text-primary)]">
              {search
                ? "No contracts match your search"
                : "No contracts tracked yet"}
            </p>
            {!search && !tagFilter && (
              <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
                Use the CLI or API to start tracking a Soroban contract, or
                click{" "}
                <Link
                  href="/contracts/new"
                  className="text-[var(--color-accent)] underline underline-offset-2 hover:opacity-80"
                >
                  run the tracking wizard
                </Link>{" "}
                to add one from here.
              </p>
            )}
          </div>
        )}

        {/* Data table */}
        {!loading && sorted.length > 0 && (
          <DataTable<ContractRow>
            columns={columns}
            data={sorted}
            rowKey={contractRowKey}
            sortColumn={sortColumn}
            sortDirection={sortDirection}
            onSort={handleSort}
            onRowClick={(c) => {
              // The detail page doesn't exist until the API confirms it.
              if (isPendingRow(c)) return;
              window.location.href = `/contracts/${c.id}`;
            }}
            emptyState={
              <div className="rounded-lg bg-[var(--color-bg-card)] px-8 py-12 text-center text-sm text-[var(--color-text-secondary)] border border-[var(--color-border)]">
                No contracts match your search.
              </div>
            }
          />
        )}

        {/* Pagination controls */}
        {!loading && contracts.length > 0 && (
          <div className="mt-4 flex items-center justify-between">
            <span className="text-xs text-[var(--color-text-secondary)]">
              Page {cursorIndex + 1}
            </span>
            <div className="flex gap-2">
              <button
                id="contracts-prev-page"
                type="button"
                onClick={handlePrev}
                disabled={isFirstPage}
                aria-label="Previous page"
                className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-40"
              >
                ← Previous
              </button>
              <button
                id="contracts-next-page"
                type="button"
                onClick={handleNext}
                disabled={isLastPage}
                aria-label="Next page"
                className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-40"
              >
                Next →
              </button>
            </div>
          </div>
        )}

        {/* Shortcut link to contract detail (accessible) */}
        <div className="sr-only">
          {sorted
            .filter((c) => !isPendingRow(c))
            .map((c) => (
              <Link key={c.id} href={`/contracts/${c.id}`}>
                {c.label ?? c.id}
              </Link>
            ))}
        </div>
      </div>
    </>
  );
}
