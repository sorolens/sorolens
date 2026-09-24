"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { DataTable, MonoId } from "@sorolens/ui";
import type { Column } from "@sorolens/ui";
import { listContracts } from "@/lib/api";
import type { ContractSummary } from "@/lib/types";
import { networkFilter, useNetwork } from "@/lib/network";
import { TableSkeleton } from "@/components/Skeleton";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 20;

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
];

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export default function ContractsPage() {
  // Selected network from the header selector.
  const { network } = useNetwork();

  // Data state
  const [contracts, setContracts] = useState<ContractSummary[]>([]);
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

  // ---------------------------------------------------------------------------
  // Data fetching
  // ---------------------------------------------------------------------------

  const load = useCallback(
    async (cursor: string | null) => {
      setLoading(true);
      try {
        const data = await listContracts({
          cursor: cursor ?? undefined,
          limit: PAGE_SIZE,
          network: networkFilter(network),
        });
        setContracts(data.contracts ?? []);
        setHasMore(data.has_more ?? false);
      } catch {
        // Backend not reachable yet. Fall through to the empty state so the
        // page still reads as "waiting for data" instead of "broken".
        setContracts([]);
        setHasMore(false);
      } finally {
        setLoading(false);
      }
    },
    [network],
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
    const av = (a as unknown as Record<string, unknown>)[sortColumn];
    const bv = (b as unknown as Record<string, unknown>)[sortColumn];
    const cmp = String(av ?? "").localeCompare(String(bv ?? ""));
    return sortDirection === "asc" ? cmp : -cmp;
  });

  const isFirstPage = cursorIndex === 0;
  const isLastPage = !hasMore;

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <>
      <div>
        {/* Header */}
        <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">
            Contracts
          </h1>

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
              {search ? "No contracts match your search" : "No contracts tracked yet"}
            </p>
            {!search && (
              <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
                Use the CLI or API to start tracking a Soroban contract, or{" "}
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
