"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { DataTable, MonoId } from "@sorolens/ui";
import type { Column } from "@sorolens/ui";
import { listContracts, trackContract, ApiError } from "@/lib/api";
import type { ContractSummary } from "@/lib/types";
import { TableSkeleton } from "@/components/Skeleton";

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

// ---------------------------------------------------------------------------
// Track Contract Modal
// ---------------------------------------------------------------------------

interface TrackModalProps {
  onClose: () => void;
  onSuccess: () => void;
}

function TrackContractModal({ onClose, onSuccess }: TrackModalProps) {
  const [contractId, setContractId] = useState("");
  const [label, setLabel] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const idError =
    contractId.length > 0 && !CONTRACT_ID_RE.test(contractId)
      ? "Contract ID must be 56 characters starting with 'C' (A–Z, 0–9 only)"
      : null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (idError || !contractId) return;

    setSubmitting(true);
    setError(null);
    try {
      await trackContract({ id: contractId, label: label || undefined });
      onSuccess();
      onClose();
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("An unexpected error occurred");
      }
    } finally {
      setSubmitting(false);
    }
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
              onChange={(e) => {
                setContractId(e.target.value.trim());
                setError(null);
              }}
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

          {error && (
            <p
              className="rounded-lg bg-red-900/20 border border-red-900/40 px-3 py-2.5 text-sm text-red-400"
              role="alert"
            >
              {error}
            </p>
          )}

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
              disabled={submitting || !!idError || !contractId}
              className="flex-1 rounded-lg bg-[var(--color-accent)] px-4 py-2.5 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity disabled:opacity-50 hover:opacity-90"
            >
              {submitting ? "Tracking…" : "Track contract"}
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

const COLUMNS: Column<ContractSummary>[] = [
  {
    key: "id",
    header: "Contract ID",
    sortable: true,
    accessor: (c) => (
      <span className="font-mono text-xs">
        <MonoId value={c.id} headChars={8} tailChars={8} />
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
        <span className="text-[var(--color-text-secondary)]">—</span>
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
];

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export default function ContractsPage() {
  // Data state
  const [contracts, setContracts] = useState<ContractSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Pagination state — stack of cursors, index 0 = first page
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

  // ---------------------------------------------------------------------------
  // Data fetching
  // ---------------------------------------------------------------------------

  const load = useCallback(async (cursor: string | null) => {
    setLoading(true);
    setError(null);
    try {
      const data = await listContracts({
        cursor: cursor ?? undefined,
        limit: PAGE_SIZE,
      });
      setContracts(data.contracts ?? []);
      setHasMore(data.has_more ?? false);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Failed to load contracts");
      }
      setContracts([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(cursors[cursorIndex]);
  }, [load, cursors, cursorIndex]);

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
    const av = (a as Record<string, unknown>)[sortColumn];
    const bv = (b as Record<string, unknown>)[sortColumn];
    const cmp = String(av ?? "").localeCompare(String(bv ?? ""));
    return sortDirection === "asc" ? cmp : -cmp;
  });

  const isFirstPage = cursorIndex === 0;
  const isLastPage = !hasMore;

  // ---------------------------------------------------------------------------
  // Track success — refresh first page
  // ---------------------------------------------------------------------------

  const handleTrackSuccess = () => {
    setCursors([null]);
    setCursorIndex(0);
  };

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <>
      {showModal && (
        <TrackContractModal
          onClose={() => setShowModal(false)}
          onSuccess={handleTrackSuccess}
        />
      )}

      <div>
        {/* Header */}
        <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">
            Contracts
          </h1>

          <button
            id="track-contract-btn"
            type="button"
            onClick={() => setShowModal(true)}
            className="inline-flex items-center gap-2 rounded-lg bg-[var(--color-accent)] px-4 py-2 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90"
          >
            <span aria-hidden="true">+</span>
            Track contract
          </button>
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

        {/* Error state */}
        {error && !loading && (
          <div className="mb-4 rounded-lg border border-red-900/40 bg-red-900/20 px-4 py-3 text-sm text-red-400">
            {error}
          </div>
        )}

        {/* Loading skeleton */}
        {loading && <TableSkeleton rows={PAGE_SIZE} />}

        {/* Empty state */}
        {!loading && !error && sorted.length === 0 && (
          <div className="rounded-lg bg-[var(--color-bg-card)] px-8 py-16 text-center border border-[var(--color-border)]">
            <p className="text-lg font-medium text-[var(--color-text-primary)]">
              {search ? "No contracts match your search" : "No contracts tracked yet"}
            </p>
            {!search && (
              <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
                Click{" "}
                <button
                  type="button"
                  onClick={() => setShowModal(true)}
                  className="text-[var(--color-accent)] underline underline-offset-2 hover:opacity-80"
                >
                  Track contract
                </button>{" "}
                to add your first Soroban contract.
              </p>
            )}
          </div>
        )}

        {/* Data table */}
        {!loading && sorted.length > 0 && (
          <DataTable<ContractSummary>
            columns={COLUMNS}
            data={sorted}
            rowKey={(c) => c.id}
            sortColumn={sortColumn}
            sortDirection={sortDirection}
            onSort={handleSort}
            onRowClick={(c) => {
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
          {sorted.map((c) => (
            <Link key={c.id} href={`/contracts/${c.id}`}>
              {c.label ?? c.id}
            </Link>
          ))}
        </div>
      </div>
    </>
  );
}
