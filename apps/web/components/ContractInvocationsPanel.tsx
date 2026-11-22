"use client";

import { useCallback, useEffect, useState } from "react";
import { DataTable, MonoId } from "@sorolens/ui";
import type { Column } from "@sorolens/ui";
import { getContractInvocations } from "@/lib/api";
import type { Invocation } from "@/lib/types";
import { TableSkeleton } from "@/components/Skeleton";
import { ErrorState, errorMessage } from "@/components/ErrorState";

interface Props {
  contractId: string;
}

const PAGE_SIZE = 20;

function formatCount(n: number): string {
  return n.toLocaleString();
}

function formatTimestamp(iso: string): string {
  try {
    return new Date(iso).toLocaleString(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return iso;
  }
}

const COLUMNS: Column<Invocation>[] = [
  {
    key: "function_name",
    header: "Function",
    accessor: (inv) =>
      inv.function_name ? (
        <span className="font-mono text-xs font-medium text-[var(--color-text-primary)]">
          {inv.function_name}
        </span>
      ) : (
        <span className="text-xs text-[var(--color-text-secondary)]">--</span>
      ),
  },
  {
    key: "status",
    header: "Status",
    accessor: (inv) => {
      const isSuccess = inv.status === "success" || inv.status === "ok";
      return (
        <span
          className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium capitalize ${
            isSuccess
              ? "bg-[var(--color-safe-bg)] text-[var(--color-safe)]"
              : "bg-[var(--color-danger-bg)]/40 text-[var(--color-danger-text)]"
          }`}
        >
          {inv.status}
        </span>
      );
    },
  },
  {
    key: "ledger",
    header: "Ledger",
    accessor: (inv) => (
      <span className="font-mono text-xs tabular-nums text-[var(--color-text-secondary)]">
        {inv.ledger.toLocaleString()}
      </span>
    ),
  },
  {
    key: "ledger_closed_at",
    header: "Time",
    accessor: (inv) => (
      <span className="text-xs text-[var(--color-text-secondary)]">
        {formatTimestamp(inv.ledger_closed_at)}
      </span>
    ),
  },
  {
    key: "cpu_insn",
    header: "CPU Instructions",
    accessor: (inv) => (
      <span className="text-xs tabular-nums">{formatCount(inv.cpu_insn)}</span>
    ),
  },
  {
    key: "mem_byte",
    header: "Memory (Bytes)",
    accessor: (inv) => (
      <span className="text-xs tabular-nums">{formatCount(inv.mem_byte)}</span>
    ),
  },
  {
    key: "tx_hash",
    header: "Transaction",
    accessor: (inv) => (
      <span className="font-mono text-xs text-[var(--color-text-secondary)]">
        <MonoId value={inv.tx_hash} headChars={6} tailChars={6} />
      </span>
    ),
  },
];

export function ContractInvocationsPanel({ contractId }: Props) {
  const [invocations, setInvocations] = useState<Invocation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreError, setMoreError] = useState<string | null>(null);

  // Filters
  const [functionFilter, setFunctionFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");

  const loadInvocations = useCallback(
    async (fnName?: string, status?: string) => {
      setLoading(true);
      setError(null);
      setMoreError(null);
      try {
        const res = await getContractInvocations(contractId, {
          limit: PAGE_SIZE,
          function_name: fnName?.trim() || undefined,
          status: status || undefined,
        });
        setInvocations(res.invocations ?? []);
        setNextCursor(res.next_cursor || null);
      } catch (err) {
        setError(errorMessage(err, "Failed to load invocations."));
      } finally {
        setLoading(false);
      }
    },
    [contractId]
  );

  useEffect(() => {
    loadInvocations(functionFilter, statusFilter);
  }, [loadInvocations, functionFilter, statusFilter]);

  const handleLoadMore = async () => {
    if (!nextCursor || loadingMore) return;
    setLoadingMore(true);
    setMoreError(null);
    try {
      const res = await getContractInvocations(contractId, {
        cursor: nextCursor,
        limit: PAGE_SIZE,
        function_name: functionFilter.trim() || undefined,
        status: statusFilter || undefined,
      });
      setInvocations((prev) => [...prev, ...(res.invocations ?? [])]);
      setNextCursor(res.next_cursor || null);
    } catch (err) {
      setMoreError(errorMessage(err, "Failed to load more invocations."));
    } finally {
      setLoadingMore(false);
    }
  };

  return (
    <div data-testid="contract-invocations-panel" className="space-y-4">
      {/* Filters Bar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-1 flex-col gap-3 sm:flex-row sm:items-center">
          <input
            id="invocations-fn-filter"
            data-testid="invocation-function-filter"
            type="search"
            placeholder="Filter by function name..."
            value={functionFilter}
            onChange={(e) => setFunctionFilter(e.target.value)}
            className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2 text-sm text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none sm:max-w-xs"
          />
          <select
            id="invocations-status-filter"
            data-testid="invocation-status-filter"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            aria-label="Filter by status"
            className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none"
          >
            <option value="">All statuses</option>
            <option value="success">Success</option>
            <option value="failed">Failed</option>
          </select>
        </div>
        {(functionFilter || statusFilter) && (
          <button
            type="button"
            onClick={() => {
              setFunctionFilter("");
              setStatusFilter("");
            }}
            className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] underline"
          >
            Clear filters
          </button>
        )}
      </div>

      {/* Content */}
      {loading ? (
        <TableSkeleton rows={5} />
      ) : error ? (
        <ErrorState
          title="Couldn't load invocations."
          message={error}
          onRetry={() => loadInvocations(functionFilter, statusFilter)}
        />
      ) : invocations.length === 0 ? (
        <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-8 text-center text-sm text-[var(--color-text-secondary)]">
          No invocations found matching the criteria.
        </div>
      ) : (
        <div className="space-y-4">
          <DataTable
            data={invocations}
            columns={COLUMNS}
            rowKey={(inv) => `${inv.tx_hash}-${inv.ledger}`}
          />
          {Boolean(nextCursor) && (
            <div className="flex justify-center pt-2">
              <button
                type="button"
                data-testid="load-more-invocations"
                disabled={loadingMore}
                onClick={handleLoadMore}
                className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] disabled:opacity-50"
              >
                {loadingMore ? "Loading..." : "Load more invocations"}
              </button>
            </div>
          )}
          {moreError && (
            <ErrorState
              compact
              title="Couldn't load more invocations."
              message={moreError}
              onRetry={handleLoadMore}
            />
          )}
        </div>
      )}
    </div>
  );
}
