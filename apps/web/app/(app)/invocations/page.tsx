"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { DataTable, MonoId } from "@sorolens/ui";
import type { Column } from "@sorolens/ui";
import { listInvocations } from "@/lib/api";
import type { Invocation } from "@/lib/types";
import { networkFilter, useNetwork } from "@/lib/network";
import { TableSkeleton } from "@/components/Skeleton";

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PAGE_SIZE = 20;

interface Filters {
  contractId: string;
  fn: string;
  since: string;
  until: string;
}

const EMPTY_FILTERS: Filters = { contractId: "", fn: "", since: "", until: "" };

function hasActiveFilters(f: Filters): boolean {
  return Boolean(f.contractId || f.fn || f.since || f.until);
}

// ---------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------

function formatTimestamp(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatCount(n: number): string {
  return n.toLocaleString();
}

// ---------------------------------------------------------------------------
// Columns
// ---------------------------------------------------------------------------

const COLUMNS: Column<Invocation>[] = [
  {
    key: "contract_id",
    header: "Contract ID",
    accessor: (inv) => (
      <span className="font-mono text-xs">
        <MonoId value={inv.contract_id} headChars={8} tailChars={8} />
      </span>
    ),
  },
  {
    key: "function_name",
    header: "Function Name",
    accessor: (inv) =>
      inv.function_name ? (
        <span className="font-mono text-xs text-[var(--color-text-primary)]">
          {inv.function_name}
        </span>
      ) : (
        <span className="text-[var(--color-text-secondary)]">--</span>
      ),
  },
  {
    key: "cpu_insn",
    header: "CPU Instructions",
    sortable: true,
    accessor: (inv) => (
      <span className="tabular-nums">{formatCount(inv.cpu_insn)}</span>
    ),
  },
  {
    key: "mem_byte",
    header: "Memory Bytes",
    accessor: (inv) => (
      <span className="tabular-nums">{formatCount(inv.mem_byte)}</span>
    ),
  },
  {
    key: "ledger_io",
    header: "Ledger I/O",
    accessor: (inv) => {
      const total = inv.ledger_read_byte + inv.ledger_write_byte;
      return (
        <span
          className="tabular-nums"
          title={`read ${formatCount(inv.ledger_read_byte)} B · write ${formatCount(inv.ledger_write_byte)} B`}
        >
          {formatCount(total)} B
        </span>
      );
    },
  },
  {
    key: "resource_fee_charged",
    header: "Fee Charged",
    sortable: true,
    accessor: (inv) => (
      <span className="tabular-nums">
        {formatCount(inv.resource_fee_charged)}
        <span className="ml-1 text-xs text-[var(--color-text-secondary)]">
          stroops
        </span>
      </span>
    ),
  },
  {
    key: "ledger",
    header: "Ledger",
    accessor: (inv) => (
      <span className="tabular-nums">{formatCount(inv.ledger)}</span>
    ),
  },
  {
    key: "ledger_closed_at",
    header: "Timestamp",
    sortable: true,
    accessor: (inv) => (
      <span className="text-xs text-[var(--color-text-secondary)]">
        {formatTimestamp(inv.ledger_closed_at)}
      </span>
    ),
  },
];

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------

export default function InvocationsPage() {
  const { network } = useNetwork();

  // Data state
  const [invocations, setInvocations] = useState<Invocation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  // Pagination: stack of cursors, index 0 is the first page.
  const [cursors, setCursors] = useState<(string | null)[]>([null]);
  const [cursorIndex, setCursorIndex] = useState(0);
  const [nextCursor, setNextCursor] = useState<string | null>(null);

  // Filter inputs (draft) and the applied filter set the request uses.
  const [draft, setDraft] = useState<Filters>(EMPTY_FILTERS);
  const [filters, setFilters] = useState<Filters>(EMPTY_FILTERS);

  // Sort state. The API returns newest first, which is the default order.
  const [sortColumn, setSortColumn] = useState("ledger_closed_at");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("desc");

  // Only the most recent load may write to state.
  const loadSeq = useRef(0);

  const load = useCallback(
    async (cursor: string | null) => {
      const seq = ++loadSeq.current;
      setLoading(true);
      try {
        const data = await listInvocations({
          cursor: cursor ?? undefined,
          limit: PAGE_SIZE,
          contract_id: filters.contractId || undefined,
          fn: filters.fn || undefined,
          since: filters.since || undefined,
          until: filters.until || undefined,
          network: networkFilter(network),
        });
        if (seq !== loadSeq.current) return;
        setInvocations(data.invocations ?? []);
        setNextCursor(data.next_cursor || null);
        setError(false);
      } catch {
        if (seq !== loadSeq.current) return;
        // Backend unreachable or erroring: show the empty state instead of a
        // stuck spinner, and let the user retry with Apply.
        setInvocations([]);
        setNextCursor(null);
        setError(true);
      } finally {
        if (seq === loadSeq.current) setLoading(false);
      }
    },
    [filters, network],
  );

  useEffect(() => {
    load(cursors[cursorIndex]);
  }, [load, cursors, cursorIndex]);

  // Reset to the first page when the global network selector changes.
  const prevNetwork = useRef(network);
  useEffect(() => {
    if (prevNetwork.current !== network) {
      prevNetwork.current = network;
      setCursors([null]);
      setCursorIndex(0);
    }
  }, [network]);

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

  const resetToFirstPage = () => {
    setCursors([null]);
    setCursorIndex(0);
  };

  const handleApply = (e: React.FormEvent) => {
    e.preventDefault();
    setFilters({
      contractId: draft.contractId.trim(),
      fn: draft.fn.trim(),
      since: draft.since,
      until: draft.until,
    });
    resetToFirstPage();
  };

  const handleClear = () => {
    setDraft(EMPTY_FILTERS);
    setFilters(EMPTY_FILTERS);
    resetToFirstPage();
  };

  const handleNext = () => {
    if (!nextCursor) return;
    const newCursors = [...cursors.slice(0, cursorIndex + 1), nextCursor];
    setCursors(newCursors);
    setCursorIndex(cursorIndex + 1);
  };

  const handlePrev = () => {
    if (cursorIndex === 0) return;
    setCursorIndex(cursorIndex - 1);
  };

  const handleSort = (col: string) => {
    if (col === sortColumn) {
      setSortDirection((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortColumn(col);
      setSortDirection("asc");
    }
  };

  // ---------------------------------------------------------------------------
  // Derived: sorted view of the current page
  // ---------------------------------------------------------------------------

  const sorted = [...invocations].sort((a, b) => {
    let cmp = 0;
    switch (sortColumn) {
      case "cpu_insn":
        cmp = a.cpu_insn - b.cpu_insn;
        break;
      case "resource_fee_charged":
        cmp = a.resource_fee_charged - b.resource_fee_charged;
        break;
      default:
        cmp =
          new Date(a.ledger_closed_at).getTime() -
          new Date(b.ledger_closed_at).getTime();
    }
    if (cmp === 0) cmp = a.tx_hash.localeCompare(b.tx_hash);
    return sortDirection === "asc" ? cmp : -cmp;
  });

  const filtersActive = hasActiveFilters(filters);
  const isFirstPage = cursorIndex === 0;
  const isLastPage = !nextCursor;

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">
          Invocations
        </h1>
        <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
          Resource usage (CPU, memory, ledger I/O and fees) for every invocation
          across all tracked contracts.
        </p>
      </div>

      {/* Filters */}
      <form
        onSubmit={handleApply}
        className="flex flex-wrap items-end gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-4"
      >
        <div className="flex flex-col gap-1">
          <label
            htmlFor="invocations-contract-filter"
            className="text-xs font-medium text-[var(--color-text-secondary)]"
          >
            Contract ID
          </label>
          <input
            id="invocations-contract-filter"
            type="text"
            value={draft.contractId}
            onChange={(e) =>
              setDraft((d) => ({ ...d, contractId: e.target.value }))
            }
            placeholder="C…"
            spellCheck={false}
            className="w-56 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-page)] px-3 py-1.5 font-mono text-xs text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
        </div>

        <div className="flex flex-col gap-1">
          <label
            htmlFor="invocations-fn-filter"
            className="text-xs font-medium text-[var(--color-text-secondary)]"
          >
            Function name
          </label>
          <input
            id="invocations-fn-filter"
            type="text"
            value={draft.fn}
            onChange={(e) => setDraft((d) => ({ ...d, fn: e.target.value }))}
            placeholder="transfer"
            spellCheck={false}
            className="w-40 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-page)] px-3 py-1.5 font-mono text-xs text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
        </div>

        <div className="flex flex-col gap-1">
          <label
            htmlFor="invocations-since-filter"
            className="text-xs font-medium text-[var(--color-text-secondary)]"
          >
            From date
          </label>
          <input
            id="invocations-since-filter"
            type="date"
            value={draft.since}
            onChange={(e) => setDraft((d) => ({ ...d, since: e.target.value }))}
            className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-page)] px-3 py-1.5 text-xs text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
        </div>

        <div className="flex flex-col gap-1">
          <label
            htmlFor="invocations-until-filter"
            className="text-xs font-medium text-[var(--color-text-secondary)]"
          >
            To date
          </label>
          <input
            id="invocations-until-filter"
            type="date"
            value={draft.until}
            onChange={(e) => setDraft((d) => ({ ...d, until: e.target.value }))}
            className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-page)] px-3 py-1.5 text-xs text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none"
          />
        </div>

        <div className="flex items-center gap-2">
          <button
            id="invocations-apply-filter"
            type="submit"
            className="rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-xs font-medium text-[var(--color-bg-page)] transition-opacity hover:opacity-90"
          >
            Apply
          </button>
          {filtersActive && (
            <button
              id="invocations-clear-filter"
              type="button"
              onClick={handleClear}
              className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-xs font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
            >
              Clear
            </button>
          )}
        </div>
      </form>

      {/* Loading skeleton */}
      {loading && <TableSkeleton rows={PAGE_SIZE} />}

      {/* Empty state */}
      {!loading && sorted.length === 0 && (
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-8 py-16 text-center">
          <p className="text-lg font-medium text-[var(--color-text-primary)]">
            {filtersActive
              ? "No invocations match your filters"
              : error
                ? "Could not load invocations"
                : "No invocations indexed yet"}
          </p>
          <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
            {filtersActive
              ? "Try widening the date range or clearing the contract and function filters."
              : error
                ? "The API could not be reached. Check NEXT_PUBLIC_API_URL and try Apply again."
                : "Invocations appear here once the indexer has traced activity for tracked contracts."}
          </p>
        </div>
      )}

      {/* Data table */}
      {!loading && sorted.length > 0 && (
        <DataTable<Invocation>
          columns={COLUMNS}
          data={sorted}
          rowKey={(inv) => `${inv.ledger}:${inv.tx_hash}`}
          sortColumn={sortColumn}
          sortDirection={sortDirection}
          onSort={handleSort}
          onRowClick={(inv) => {
            window.location.href = `/contracts/${inv.contract_id}`;
          }}
          emptyState={
            <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-8 py-12 text-center text-sm text-[var(--color-text-secondary)]">
              No invocations match your filters.
            </div>
          }
        />
      )}

      {/* Pagination */}
      {!loading && invocations.length > 0 && (
        <div className="flex items-center justify-between">
          <span className="text-xs text-[var(--color-text-secondary)]">
            Page {cursorIndex + 1}
            {sortColumn !== "ledger_closed_at" && (
              <span className="ml-2 opacity-80">
                (sorting applies to the current page)
              </span>
            )}
          </span>
          <div className="flex gap-2">
            <button
              id="invocations-prev-page"
              type="button"
              onClick={handlePrev}
              disabled={isFirstPage}
              aria-label="Previous page"
              className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-accent)] hover:text-[var(--color-text-primary)] disabled:cursor-not-allowed disabled:opacity-40"
            >
              ← Previous
            </button>
            <button
              id="invocations-next-page"
              type="button"
              onClick={handleNext}
              disabled={isLastPage}
              aria-label="Next page"
              className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-accent)] hover:text-[var(--color-text-primary)] disabled:cursor-not-allowed disabled:opacity-40"
            >
              Next →
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
