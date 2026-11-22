"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useDebounce } from "use-debounce";
import { DataTable, Toast } from "@sorolens/ui";
import type { Column } from "@sorolens/ui";
import { LabelledId } from "@/components/LabelledId";
import { useContracts } from "@/hooks/useContracts";
import { useQueryState } from "@/hooks/useQueryState";
import { errorMessage } from "@/components/ErrorState";
import { EmptyState } from "@/components/EmptyState";
import { useLocale } from "@/lib/i18n";
import { ApiError, batchContracts } from "@/lib/api";
import type { BatchContractsAction } from "@/lib/types";
import { networkFilter, useNetwork } from "@/lib/network";
import { contractRowKey, isPendingRow } from "@/lib/optimisticTrack";
import type { ContractRow } from "@/lib/optimisticTrack";
import { useSessionUserId } from "@/lib/useSession";
import { TableSkeleton } from "@/components/Skeleton";
import ImportContractsCsv from "@/components/ImportContractsCsv";
import {
  parseContractsUrlState,
  serializeContractsUrlState,
  type ContractsUrlState,
} from "@/lib/contractsUrl";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 20;

// Sort, filters and pagination live in the URL (?sort=&dir=&q=&tag=&cursor=
// &page=) so a view is shareable (#175 for sort, #618 for the rest). The
// parse/serialize logic lives in lib/contractsUrl.ts.

// Paste-to-add (#178): a Soroban contract id is 56 chars, starts with 'C', and
// uses the base32 alphabet. Mirrors the tracking wizard's client-side check.
const PASTE_CONTRACT_ID_RE = /^C[A-Z0-9]{55}$/;

// The bulk "Compare" action (#607) links into /compare. The whole selection
// travels in the URL; the compare page caps the view at four contracts and
// tells the user about any extras, so the list does not drop ids silently.
const MIN_COMPARE = 2;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function StatusBadge({ status }: { status: string }) {
  const colorMap: Record<string, string> = {
    active: "bg-[var(--color-safe-bg)] text-[var(--color-safe-text)]",
    backfilling: "bg-[var(--color-info-bg)] text-[var(--color-info-text)]",
    error: "bg-[var(--color-danger-bg)] text-[var(--color-danger-text)]",
    inactive: "bg-[var(--color-warning-bg)] text-[var(--color-warning-text)]",
  };
  const cls =
    colorMap[status] ??
    "bg-[var(--color-warning-bg)] text-[var(--color-warning-text)]";
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
          className="flex-1 rounded-lg bg-[var(--color-danger)] px-4 py-2.5 text-sm font-semibold text-[var(--color-danger-foreground)] transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
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
            className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2.5 text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
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
 * need the click handler; the handler only calls `applyTagFilter`, which is
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

export default function ContractsClient() {
  // Selected network from the header selector.
  const { network } = useNetwork();
  const userId = useSessionUserId();
  const { t } = useLocale();

  // Still needed for the paste-to-add toast's navigation to the wizard.
  const router = useRouter();

  // The shareable list state (sort, filters, current page) lives in the
  // URL. The server page seeds its SWR fallback from the same params, so
  // the first render and the fallback agree.
  const [applied, setApplied] = useQueryState<ContractsUrlState>({
    parse: parseContractsUrlState,
    serialize: serializeContractsUrlState,
  });
  const sortColumn = applied.sort;
  const sortDirection = applied.dir;
  const tagFilter = applied.tag;

  // Free-text search filters the current page client-side. The input is
  // local so typing stays instant; the URL catches up debounced (replace,
  // so it does not spam history) to keep the view shareable.
  const [searchInput, setSearchInput] = useState(applied.q);
  const [debouncedSearchInput] = useDebounce(searchInput, 400);
  useEffect(() => {
    setApplied(
      (prev) =>
        prev.q === debouncedSearchInput
          ? prev
          : { ...prev, q: debouncedSearchInput },
      "replace"
    );
  }, [debouncedSearchInput, setApplied]);
  // When the q param arrives from the URL (reload, back/forward), the
  // input follows it.
  useEffect(() => {
    setSearchInput(applied.q);
  }, [applied.q]);

  // Same treatment for the server-side tag filter: the input is local and
  // debounced into the URL; chip clicks write through immediately.
  const [tagInput, setTagInput] = useState(applied.tag);
  const [debouncedTagInput] = useDebounce(tagInput, 400);
  useEffect(() => {
    setApplied(
      (prev) =>
        prev.tag === debouncedTagInput
          ? prev
          : { ...prev, tag: debouncedTagInput },
      "replace"
    );
  }, [debouncedTagInput, setApplied]);
  useEffect(() => {
    setTagInput(applied.tag);
  }, [applied.tag]);

  const applyTagFilter = useCallback(
    (tag: string) => {
      setTagInput(tag);
      // A new tag changes the result set, so pagination restarts in the
      // same write rather than relying on the reset effect's second
      // navigation.
      setApplied(
        (prev) =>
          prev.tag === tag && prev.cursor === "" && prev.page === 1
            ? prev
            : { ...prev, tag, cursor: "", page: 1 },
        "push"
      );
    },
    [setApplied]
  );

  // Pagination: stack of cursors, index 0 = first page. The URL holds the
  // current cursor; the stack is session memory and collapses to the
  // current page whenever the cursor arrives from the URL instead of our
  // own pagination (reload, back, shared link).
  const [cursors, setCursors] = useState<(string | null)[]>([
    applied.cursor || null,
  ]);
  useEffect(() => {
    setCursors((stack) =>
      stack[stack.length - 1] === (applied.cursor || null)
        ? stack
        : [applied.cursor || null]
    );
  }, [applied.cursor]);

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
    message: ReactNode;
    variant: "info" | "success" | "error";
  } | null>(null);
  const toastSeq = useRef(0);
  const dismissToast = useCallback(() => setToast(null), []);

  // Paste-to-add contract (#178). Pasting a valid Soroban contract id anywhere
  // on the page surfaces a "Track this contract?" toast whose Confirm routes to
  // the tracking wizard with the id prefilled (so the network step still runs).
  // Anything that is not a valid id is ignored silently.
  useEffect(() => {
    const onPaste = (e: ClipboardEvent) => {
      const text = e.clipboardData?.getData("text")?.trim().toUpperCase();
      if (!text || !PASTE_CONTRACT_ID_RE.test(text)) return;
      setToast({
        id: ++toastSeq.current,
        variant: "info",
        message: (
          <span>
            Track this contract?{" "}
            <button
              type="button"
              data-testid="paste-track-confirm"
              onClick={() => {
                setToast(null);
                router.push(`/contracts/new?id=${encodeURIComponent(text)}`);
              }}
              style={{
                background: "none",
                border: "none",
                color: "inherit",
                cursor: "pointer",
                font: "inherit",
                fontWeight: 600,
                padding: 0,
                textDecoration: "underline",
              }}
            >
              Confirm
            </button>
          </span>
        ),
      });
    };
    document.addEventListener("paste", onPaste);
    return () => document.removeEventListener("paste", onPaste);
  }, [router]);

  // ---------------------------------------------------------------------------
  // Data fetching (SWR)
  // ---------------------------------------------------------------------------

  // Every (cursor, network, tag) query is its own SWR cache entry: a visited
  // page renders instantly from cache and revalidates in the background, and
  // duplicate requests for the same query are de-duplicated. When the API is
  // unreachable `error` is set and an error state with a retry replaces the
  // empty state, so an outage is never mistaken for "no contracts".
  const { data, error, isLoading, isValidating, mutate } = useContracts({
    cursor: applied.cursor || undefined,
    limit: PAGE_SIZE,
    network: networkFilter(network),
    tag: tagFilter || undefined,
    sort: sortColumn,
    dir: sortDirection,
  });

  const contracts: ContractRow[] = data?.contracts ?? [];
  const hasMore = Boolean(
    (data?.next_cursor && data.next_cursor !== "") || data?.has_more
  );
  const loading = isLoading;
  const loadFailed = !loading && Boolean(error);

  // Reset to the first page when the network, tag filter or sort changes:
  // an in-flight cursor was produced under the previous query and no
  // longer points at the next page. The ref guard keeps this from firing
  // on mount; the change itself also alters the SWR key, which triggers
  // the fetch on its own.
  const prevQueryKey = useRef(
    `${network}|${tagFilter}|${sortColumn}:${sortDirection}`
  );
  useEffect(() => {
    const key = `${network}|${tagFilter}|${sortColumn}:${sortDirection}`;
    if (prevQueryKey.current === key) return;
    prevQueryKey.current = key;
    setApplied(
      (prev) =>
        prev.cursor === "" && prev.page === 1
          ? prev
          : { ...prev, cursor: "", page: 1 },
      "replace"
    );
    setCursors([null]);
  }, [network, tagFilter, sortColumn, sortDirection, setApplied]);

  // ---------------------------------------------------------------------------
  // Pagination handlers
  // ---------------------------------------------------------------------------

  const handleNext = () => {
    if (!hasMore) return;
    const nextCursor =
      data?.next_cursor ||
      (contracts.length > 0 ? contracts[contracts.length - 1].id : null);
    if (!nextCursor) return;
    setApplied((prev) => ({
      ...prev,
      cursor: nextCursor,
      page: prev.page + 1,
    }));
    setCursors((stack) => [...stack, nextCursor]);
  };

  const handlePrev = () => {
    if (cursors.length <= 1) return;
    setApplied((prev) => ({
      ...prev,
      cursor: cursors[cursors.length - 2] ?? "",
      page: Math.max(1, prev.page - 1),
    }));
    setCursors((stack) => stack.slice(0, -1));
  };

  // ---------------------------------------------------------------------------
  // Sort handler
  // ---------------------------------------------------------------------------

  const handleSort = (col: string) => {
    // Same column flips the direction; a new column starts ascending. The
    // new sort lands in the URL (shareable, #175) and the reset effect
    // above restarts pagination.
    setApplied(
      (prev) =>
        col === prev.sort
          ? { ...prev, dir: prev.dir === "asc" ? "desc" : "asc" }
          : { ...prev, sort: col as ContractsUrlState["sort"], dir: "asc" },
      "replace"
    );
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

  // Bulk "Compare" entry point (#607): the selection travels in the shared
  // /compare?ids= URL, which validates and renders it on first paint.
  const compareHref = `/compare?ids=${encodeURIComponent(
    Array.from(selected).join(",")
  )}`;

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
        userId
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
      // The list changed, so revalidate the page the user is looking at.
      void mutate();
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
    if (!searchInput) return true;
    const q = searchInput.toLowerCase();
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

  const isFirstPage = cursors.length <= 1;
  const isLastPage = !hasMore;

  // ---------------------------------------------------------------------------
  // Track success: refresh first page
  // ---------------------------------------------------------------------------

  const handleTrackSuccess = () => {
    setApplied(
      (prev) =>
        prev.cursor === "" && prev.page === 1
          ? prev
          : { ...prev, cursor: "", page: 1 },
      "replace"
    );
    setCursors([null]);
    // Revalidate the visible page so a newly tracked/imported contract shows
    // up even when we were already on page 1 (the SWR key does not change).
    void mutate();
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
    return [selectColumn, ...makeColumns((tag) => applyTagFilter(tag))];
  }, [allSelected, pageIds, selected, toggleAll, toggleRow]);

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <>
      {showImport && (
        <ImportContractsCsv
          network={network}
          userId={userId}
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
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder="Search by alias or contract ID…"
            aria-label="Search contracts"
            className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-2.5 text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
          <input
            id="contracts-tag-filter"
            type="search"
            value={tagInput}
            onChange={(e) => setTagInput(e.target.value)}
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
              {selected.size >= MIN_COMPARE ? (
                <Link
                  id="bulk-compare-btn"
                  href={compareHref}
                  className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)] hover:border-[var(--color-accent)]"
                >
                  Compare {selected.size} contracts
                </Link>
              ) : (
                <button
                  id="bulk-compare-btn"
                  type="button"
                  disabled
                  title={`Select at least ${MIN_COMPARE} contracts to compare`}
                  className="rounded-lg border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-secondary)] opacity-50"
                >
                  Compare
                </button>
              )}
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
                className="rounded-lg border border-[var(--color-danger-border)] bg-[var(--color-danger-bg)] px-3 py-1.5 text-sm font-medium text-[var(--color-danger-text)] transition-colors hover:bg-[var(--color-danger-bg)] disabled:cursor-not-allowed disabled:opacity-50"
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

        {/* Failed request: keeps any previously loaded rows visible below */}
        {loadFailed && (
          <EmptyState
            variant="error"
            className="mb-4"
            title={t("contracts.error.title")}
            message={errorMessage(error, "The contracts request failed.")}
            onRetry={() => void mutate()}
            retrying={isValidating}
          />
        )}

        {/* Empty state: only when the request succeeded */}
        {!loading && !loadFailed && sorted.length === 0 && (
          <EmptyState
            variant={searchInput ? "no-results" : "first-run"}
            title={
              searchInput
                ? t("contracts.empty.noResults.title")
                : t("contracts.empty.firstRun.title")
            }
            description={
              !searchInput && !tagFilter ? (
                <>
                  {t("contracts.empty.firstRun.body")}{" "}
                  <Link
                    href="/contracts/new"
                    className="text-[var(--color-accent)] underline underline-offset-2 hover:opacity-80"
                  >
                    {t("contracts.empty.firstRun.action")}
                  </Link>{" "}
                  {t("contracts.empty.firstRun.bodySuffix")}
                </>
              ) : undefined
            }
          />
        )}

        {/* Data table */}
        {!loading && sorted.length > 0 && (
          <DataTable<ContractRow>
            columns={columns}
            data={sorted}
            virtualized
            rowKey={contractRowKey}
            sortColumn={sortColumn}
            sortDirection={sortDirection}
            onSort={handleSort}
            // A real link in the ID cell (issue #610) is keyboard reachable
            // and announced by screen readers. Pending rows get no link: the
            // detail page does not exist until the API confirms the contract.
            linkColumnKey="id"
            linkComponent={Link}
            rowHref={(c) => (isPendingRow(c) ? null : `/contracts/${c.id}`)}
            emptyState={
              <EmptyState
                variant="no-results"
                compact
                title={t("contracts.empty.noResults.body")}
              />
            }
          />
        )}

        {/* Pagination controls */}
        {!loading && contracts.length > 0 && (
          <div className="mt-4 flex items-center justify-between">
            <span className="text-xs text-[var(--color-text-secondary)]">
              Page {applied.page}
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
      </div>
    </>
  );
}
