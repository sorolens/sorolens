"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { DataTable, Toast } from "@sorolens/ui";
import type { Column } from "@sorolens/ui";
import { listContracts } from "@/lib/api";
import type { TrackContractRequest } from "@/lib/types";
import { networkFilter, useNetwork } from "@/lib/network";
import {
  contractRowKey,
  isPendingRow,
  trackContractOptimistically,
} from "@/lib/optimisticTrack";
import type { ContractRow } from "@/lib/optimisticTrack";
import { TableSkeleton } from "@/components/Skeleton";
import ImportContractsCsv from "@/components/ImportContractsCsv";
import { LabelledId } from "@/components/LabelledId";

// RBAC identity: same localStorage key the watchlist page uses, so the UI
// registers a contract under the same user identity. Must map to a user
// granted at least the contributor role in the API's users table.
const STORAGE_KEY = "sorolens_user_id";

function getUserId(): string {
  if (typeof window === "undefined") return "";
  try {
    return window.localStorage.getItem(STORAGE_KEY) || "";
  } catch {
    // localStorage can be unavailable (private mode, some test runners);
    // RBAC still allows registered-contract calls for anonymous callers as
    // reads remain open.
    return "";
  }
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 20;

// Soroban contract IDs are 56-char strkeys starting with 'C'
const CONTRACT_ID_RE = /^C[A-Z0-9]{55}$/;

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
// Track Contract Modal
// ---------------------------------------------------------------------------

interface TrackModalProps {
  onClose: () => void;
  /** Receives validated input; the page performs the (optimistic) API call. */
  onSubmit: (input: TrackContractRequest) => void;
}

function TrackContractModal({ onClose, onSubmit }: TrackModalProps) {
  const [contractId, setContractId] = useState("");
  const [label, setLabel] = useState("");

  const idError =
    contractId.length > 0 && !CONTRACT_ID_RE.test(contractId)
      ? "Contract ID must be 56 characters starting with 'C' (A–Z, 0–9 only)"
      : null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (idError || !contractId) return;

    // Close right away so the optimistic row is visible; API errors are
    // reported by the page with a toast.
    onSubmit({ id: contractId, label: label || undefined });
    onClose();
  };

  // Close on backdrop click
  const backdropRef = useRef<HTMLDivElement>(null);
  const handleBackdrop = (e: React.MouseEvent) => {
    if (e.target === backdropRef.current) onClose();
  };

  // Close on Escape
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
      onClick={handleBackdrop}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      aria-modal="true"
      role="dialog"
      aria-labelledby="track-modal-title"
    >
      <div className="w-full max-w-md rounded-xl bg-[var(--color-bg-card)] p-6 shadow-2xl border border-[var(--color-border)]">
        <div className="mb-5 flex items-center justify-between">
          <h2
            id="track-modal-title"
            className="text-lg font-semibold text-[var(--color-text-primary)]"
          >
            Track a Contract
          </h2>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors"
            aria-label="Close modal"
            title="Close modal"
          >
            ✕
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4" noValidate>
          <div>
            <label
              htmlFor="track-contract-id"
              className="mb-1.5 block text-sm font-medium text-[var(--color-text-secondary)]"
            >
              Contract ID <span className="text-red-400">*</span>
            </label>
            <input
              id="track-contract-id"
              type="text"
              value={contractId}
              onChange={(e) => setContractId(e.target.value.trim())}
              placeholder="C…"
              className="w-full rounded-lg border border-[var(--color-border)] bg-black/30 px-3 py-2.5 font-mono text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
              autoComplete="off"
              spellCheck={false}
              required
            />
            {idError && (
              <p className="mt-1.5 text-xs text-red-400" role="alert">
                {idError}
              </p>
            )}
          </div>

          <div>
            <label
              htmlFor="track-contract-label"
              className="mb-1.5 block text-sm font-medium text-[var(--color-text-secondary)]"
            >
              Alias / Label{" "}
              <span className="text-[var(--color-text-secondary)] font-normal">
                (optional)
              </span>
            </label>
            <input
              id="track-contract-label"
              type="text"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="My Contract"
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
              id="track-modal-submit"
              type="submit"
              disabled={!!idError || !contractId}
              className="flex-1 rounded-lg bg-[var(--color-accent)] px-4 py-2.5 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity disabled:opacity-50 hover:opacity-90"
            >
              Track contract
            </button>
          </div>
        </form>
      </div>
    </div>
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
    key: "label",
    header: "Alias",
    sortable: true,
    accessor: (c) =>
      c.label ? (
        <span className="text-[var(--color-text-primary)]">{c.label}</span>
      ) : (
        <span className="text-[var(--color-text-secondary)]">--</span>
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

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export default function ContractsPage() {
  // Selected network from the header selector.
  const { network } = useNetwork();

  // Data state
  const [contracts, setContracts] = useState<ContractRow[]>([]);
  const [loading, setLoading] = useState(true);

  // Pagination state: stack of cursors, index 0 = first page
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const [cursorIndex, setCursorIndex] = useState(0);
  const [hasMore, setHasMore] = useState(false);

  // Search state
  const [search, setSearch] = useState("");

  // Sort state
  const [sortColumn, setSortColumn] = useState<string>("added_at");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("desc");

  // Modal state
  const [showModal, setShowModal] = useState(false);
  const [showImport, setShowImport] = useState(false);

  // Track state: one request in flight at a time, errors surface as a toast.
  const [trackPending, setTrackPending] = useState(false);
  const [toast, setToast] = useState<{ id: number; message: string } | null>(
    null
  );
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
    [network]
  );

  useEffect(() => {
    load(cursors[cursorIndex]);
  }, [load, cursors, cursorIndex]);

  // Reset to the first page when the network filter changes. The ref guard
  // keeps this from firing an extra fetch on mount.
  const prevNetwork = useRef(network);
  useEffect(() => {
    if (prevNetwork.current !== network) {
      prevNetwork.current = network;
      setCursors([null]);
      setCursorIndex(0);
    }
  }, [network]);

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

  // ---------------------------------------------------------------------------
  // Track submit: optimistic prepend, rollback + toast on API error
  // ---------------------------------------------------------------------------

  const handleTrackSubmit = async (input: TrackContractRequest) => {
    setTrackPending(true);

    // A load() still in flight would land after the prepend and wipe the
    // optimistic row, so supersede it and show the current list instead.
    const interruptedLoad = loading;
    if (interruptedLoad) {
      loadSeq.current++;
      setLoading(false);
    }
    const seqAtSubmit = loadSeq.current;
    const listIsCurrent = () => loadSeq.current === seqAtSubmit;

    const result = await trackContractOptimistically(
      contracts,
      setContracts,
      input,
      {
        userId: getUserId(),
        network: networkFilter(network),
        // If the user paged or switched network meanwhile, a newer load()
        // already replaced the list; restoring the snapshot would clobber it.
        shouldRollback: listIsCurrent,
      }
    );
    setTrackPending(false);

    if (result.ok) {
      handleTrackSuccess();
      return;
    }
    setToast({ id: ++toastSeq.current, message: result.message });
    // The superseded load never landed, so fetch the page it was loading.
    if (interruptedLoad && listIsCurrent()) load(cursors[cursorIndex]);
  };

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <>
      {showModal && (
        <TrackContractModal
          onClose={() => setShowModal(false)}
          onSubmit={handleTrackSubmit}
        />
      )}

      {showImport && (
        <ImportContractsCsv
          network={network}
          userId={getUserId()}
          onClose={() => setShowImport(false)}
          onImported={handleTrackSuccess}
        />
      )}

      {toast && (
        <Toast
          key={toast.id}
          message={toast.message}
          variant="error"
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
            <button
              id="track-contract-btn"
              type="button"
              onClick={() => setShowModal(true)}
              disabled={trackPending}
              className="inline-flex items-center gap-2 rounded-lg bg-[var(--color-accent)] px-4 py-2 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
            >
              <span aria-hidden="true">+</span>
              {trackPending ? "Tracking…" : "Track contract"}
            </button>
          </div>
        </div>

        {/* Search */}
        <div className="mb-4">
          <input
            id="contracts-search"
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by alias or contract ID…"
            aria-label="Search contracts"
            className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-2.5 text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
        </div>

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
            {!search && (
              <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
                Use the CLI or API to start tracking a Soroban contract, or
                click{" "}
                <button
                  type="button"
                  onClick={() => setShowModal(true)}
                  disabled={trackPending}
                  className="text-[var(--color-accent)] underline underline-offset-2 hover:opacity-80"
                >
                  Track contract
                </button>{" "}
                to add one from here.
              </p>
            )}
          </div>
        )}

        {/* Data table */}
        {!loading && sorted.length > 0 && (
          <DataTable<ContractRow>
            columns={COLUMNS}
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
