"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { MonoId } from "@sorolens/ui";
import { TableSkeleton } from "@/components/Skeleton";
import { ErrorState, errorMessage } from "@/components/ErrorState";
import {
  DEFAULT_TOKEN_DECIMALS,
  filterHolders,
  filterTransfers,
  formatTokenAmount,
  getTokenHolders,
  getTokenTransfers,
  holderDistribution,
  holdersToCsv,
  parseTokenAmount,
  shareOfSupply,
  supplyTotal,
  transfersToCsv,
  type TokenHolder,
  type TokenTransfer,
} from "@/lib/tokens";

const PAGE_SIZE = 25;

const INPUT_CLASS =
  "rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-1.5 text-sm";
const BUTTON_CLASS =
  "rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:border-[var(--color-accent)] hover:text-[var(--color-text-primary)] disabled:opacity-50";
const HEADER_CLASS =
  "border-b border-[var(--color-border)] text-left text-xs text-[var(--color-text-secondary)]";

function downloadCsv(filename: string, csv: string) {
  const url = URL.createObjectURL(new Blob([csv], { type: "text/csv" }));
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}

interface PageResult<T> {
  rows: T[];
  nextCursor: string;
  decimals?: number;
  totalSupply?: string;
}

/** Cursor pagination shared by the transfers and holders tables. */
function useTokenPages<T>(
  load: (cursor?: string) => Promise<PageResult<T>>,
  failure: string
) {
  const [rows, setRows] = useState<T[]>([]);
  const [cursor, setCursor] = useState("");
  const [decimals, setDecimals] = useState(DEFAULT_TOKEN_DECIMALS);
  const [totalSupply, setTotalSupply] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [moreError, setMoreError] = useState<string | null>(null);
  const [reload, setReload] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    load()
      .then((page) => {
        if (cancelled) return;
        setRows(page.rows);
        setCursor(page.nextCursor);
        setDecimals(page.decimals ?? DEFAULT_TOKEN_DECIMALS);
        setTotalSupply(page.totalSupply);
      })
      .catch((err) => {
        if (!cancelled) setError(errorMessage(err, failure));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [load, failure, reload]);

  const loadMore = useCallback(async () => {
    setLoadingMore(true);
    setMoreError(null);
    try {
      const page = await load(cursor);
      setRows((prev) => [...prev, ...page.rows]);
      setCursor(page.nextCursor);
    } catch (err) {
      setMoreError(errorMessage(err, failure));
    } finally {
      setLoadingMore(false);
    }
  }, [load, cursor, failure]);

  return {
    rows,
    decimals,
    totalSupply,
    hasMore: Boolean(cursor),
    loading,
    loadingMore,
    error,
    moreError,
    loadMore,
    retry: () => setReload((n) => n + 1),
  };
}

function Filters({
  idPrefix,
  amountLabel,
  address,
  amount,
  amountInvalid,
  onAddress,
  onAmount,
  onExport,
  exportDisabled,
}: {
  idPrefix: string;
  amountLabel: string;
  address: string;
  amount: string;
  amountInvalid: boolean;
  onAddress: (value: string) => void;
  onAmount: (value: string) => void;
  onExport: () => void;
  exportDisabled: boolean;
}) {
  return (
    <div className="mb-4 flex flex-wrap items-end gap-3">
      <label className="flex flex-col gap-1 text-xs text-[var(--color-text-secondary)]">
        Address
        <input
          id={`${idPrefix}-address`}
          value={address}
          onChange={(e) => onAddress(e.target.value)}
          placeholder="G… or C…"
          className={INPUT_CLASS}
        />
      </label>
      <label className="flex flex-col gap-1 text-xs text-[var(--color-text-secondary)]">
        {amountLabel}
        <input
          id={`${idPrefix}-amount`}
          value={amount}
          onChange={(e) => onAmount(e.target.value)}
          inputMode="decimal"
          placeholder="0"
          aria-invalid={amountInvalid}
          className={INPUT_CLASS}
        />
      </label>
      <button
        type="button"
        onClick={onExport}
        disabled={exportDisabled}
        className={BUTTON_CLASS}
      >
        Export CSV
      </button>
      {amountInvalid && (
        <p role="alert" className="text-xs text-[var(--color-danger)]">
          Enter a valid amount.
        </p>
      )}
    </div>
  );
}

function LoadMore({
  hasMore,
  loading,
  error,
  onLoadMore,
  label,
}: {
  hasMore: boolean;
  loading: boolean;
  error: string | null;
  onLoadMore: () => void;
  label: string;
}) {
  return (
    <>
      {hasMore && (
        <button
          type="button"
          onClick={onLoadMore}
          disabled={loading}
          className={`mt-4 ${BUTTON_CLASS}`}
        >
          {loading ? "Loading…" : "Load more"}
        </button>
      )}
      {error && (
        <ErrorState
          compact
          className="mt-4"
          title={`Couldn't load more ${label}.`}
          message={error}
          onRetry={onLoadMore}
        />
      )}
    </>
  );
}

function TransfersSection({ contractId }: { contractId: string }) {
  const load = useCallback(
    async (cursor?: string) => {
      const page = await getTokenTransfers(contractId, {
        cursor,
        limit: PAGE_SIZE,
      });
      return {
        rows: page.transfers,
        nextCursor: page.next_cursor,
        decimals: page.decimals,
      };
    },
    [contractId]
  );
  const pages = useTokenPages<TokenTransfer>(
    load,
    "The transfers request failed."
  );
  const [address, setAddress] = useState("");
  const [amount, setAmount] = useState("");

  const minAmount = amount.trim()
    ? parseTokenAmount(amount, pages.decimals)
    : null;
  const amountInvalid = Boolean(amount.trim()) && minAmount === null;
  const visible = useMemo(
    () => filterTransfers(pages.rows, { address, minAmount }),
    [pages.rows, address, minAmount]
  );

  return (
    <section aria-labelledby="token-transfers-heading">
      <h2 id="token-transfers-heading" className="mb-4 text-xl font-semibold">
        Transfers
      </h2>
      {pages.loading ? (
        <TableSkeleton />
      ) : pages.error ? (
        <ErrorState
          title="Couldn't load transfers."
          message={pages.error}
          onRetry={pages.retry}
        />
      ) : (
        <>
          <Filters
            idPrefix="transfers"
            amountLabel="Minimum amount"
            address={address}
            amount={amount}
            amountInvalid={amountInvalid}
            onAddress={setAddress}
            onAmount={setAmount}
            exportDisabled={visible.length === 0}
            onExport={() =>
              downloadCsv(
                `${contractId}-transfers.csv`,
                transfersToCsv(visible, pages.decimals)
              )
            }
          />
          {visible.length === 0 ? (
            <p className="rounded-lg bg-[var(--color-bg-card)] p-6 text-sm text-[var(--color-text-secondary)]">
              {pages.rows.length === 0
                ? "No transfers indexed yet."
                : "No loaded transfers match these filters."}
            </p>
          ) : (
            <div className="overflow-x-auto rounded-lg bg-[var(--color-bg-card)]">
              <table className="w-full text-sm">
                <thead>
                  <tr className={HEADER_CLASS}>
                    <th className="px-4 py-3 font-medium">Type</th>
                    <th className="px-4 py-3 font-medium">From</th>
                    <th className="px-4 py-3 font-medium">To</th>
                    <th className="px-4 py-3 text-right font-medium">Amount</th>
                    <th className="px-4 py-3 font-medium">Transaction</th>
                    <th className="px-4 py-3 text-right font-medium">Ledger</th>
                  </tr>
                </thead>
                <tbody>
                  {visible.map((t) => (
                    <tr
                      key={t.id}
                      className="border-b border-[var(--color-border)] last:border-b-0"
                    >
                      <td className="px-4 py-3 text-xs capitalize">{t.kind}</td>
                      <td className="px-4 py-3 font-mono text-xs">
                        {t.from ? <MonoId value={t.from} /> : "--"}
                      </td>
                      <td className="px-4 py-3 font-mono text-xs">
                        {t.to ? <MonoId value={t.to} /> : "--"}
                      </td>
                      <td className="px-4 py-3 text-right font-mono text-xs">
                        {formatTokenAmount(t.amount, pages.decimals)}
                      </td>
                      <td className="px-4 py-3 font-mono text-xs">
                        <Link
                          href={`/search?q=${encodeURIComponent(t.tx_hash)}`}
                          className="text-[var(--color-accent)] hover:underline"
                        >
                          {t.tx_hash.slice(0, 10)}…
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-right font-mono text-xs">
                        {t.ledger.toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <LoadMore
            hasMore={pages.hasMore}
            loading={pages.loadingMore}
            error={pages.moreError}
            onLoadMore={pages.loadMore}
            label="transfers"
          />
        </>
      )}
    </section>
  );
}

function HoldersSection({ contractId }: { contractId: string }) {
  const load = useCallback(
    async (cursor?: string) => {
      const page = await getTokenHolders(contractId, {
        cursor,
        limit: PAGE_SIZE,
      });
      return {
        rows: page.holders,
        nextCursor: page.next_cursor,
        decimals: page.decimals,
        totalSupply: page.total_supply,
      };
    },
    [contractId]
  );
  const pages = useTokenPages<TokenHolder>(load, "The holders request failed.");
  const [address, setAddress] = useState("");
  const [amount, setAmount] = useState("");

  const minAmount = amount.trim()
    ? parseTokenAmount(amount, pages.decimals)
    : null;
  const amountInvalid = Boolean(amount.trim()) && minAmount === null;
  const visible = useMemo(
    () => filterHolders(pages.rows, { address, minAmount }),
    [pages.rows, address, minAmount]
  );
  const total = useMemo(
    () => supplyTotal(pages.rows, pages.totalSupply),
    [pages.rows, pages.totalSupply]
  );
  const distribution = useMemo(
    () => holderDistribution(pages.rows, total),
    [pages.rows, total]
  );

  return (
    <section aria-labelledby="token-holders-heading">
      <h2 id="token-holders-heading" className="mb-4 text-xl font-semibold">
        Holders
      </h2>
      {pages.loading ? (
        <TableSkeleton />
      ) : pages.error ? (
        <ErrorState
          title="Couldn't load holders."
          message={pages.error}
          onRetry={pages.retry}
        />
      ) : pages.rows.length === 0 ? (
        <p className="rounded-lg bg-[var(--color-bg-card)] p-6 text-sm text-[var(--color-text-secondary)]">
          No holders indexed yet.
        </p>
      ) : (
        <>
          <div
            data-testid="holder-distribution"
            className="mb-6 rounded-lg bg-[var(--color-bg-card)] p-4"
          >
            <h3 className="mb-3 text-sm font-medium text-[var(--color-text-secondary)]">
              Holder distribution (% of supply)
            </h3>
            <ResponsiveContainer width="100%" height={240}>
              <BarChart data={distribution}>
                <CartesianGrid
                  strokeDasharray="3 3"
                  stroke="var(--color-border)"
                />
                <XAxis
                  dataKey="label"
                  tick={{ fontSize: 11, fill: "var(--color-text-secondary)" }}
                  tickLine={false}
                  axisLine={false}
                />
                <YAxis
                  tick={{ fontSize: 11, fill: "var(--color-text-secondary)" }}
                  tickLine={false}
                  axisLine={false}
                  width={40}
                />
                <Tooltip
                  formatter={(value) => [`${value}%`, "Share"]}
                  contentStyle={{
                    background: "var(--color-bg-card)",
                    border: "1px solid var(--color-border)",
                    borderRadius: "0.5rem",
                    fontSize: "0.875rem",
                  }}
                />
                <Bar dataKey="share" fill="var(--color-accent)" />
              </BarChart>
            </ResponsiveContainer>
          </div>
          <Filters
            idPrefix="holders"
            amountLabel="Minimum balance"
            address={address}
            amount={amount}
            amountInvalid={amountInvalid}
            onAddress={setAddress}
            onAmount={setAmount}
            exportDisabled={visible.length === 0}
            onExport={() =>
              downloadCsv(
                `${contractId}-holders.csv`,
                holdersToCsv(visible, pages.decimals, total)
              )
            }
          />
          {visible.length === 0 ? (
            <p className="rounded-lg bg-[var(--color-bg-card)] p-6 text-sm text-[var(--color-text-secondary)]">
              No loaded holders match these filters.
            </p>
          ) : (
            <div className="overflow-x-auto rounded-lg bg-[var(--color-bg-card)]">
              <table className="w-full text-sm">
                <thead>
                  <tr className={HEADER_CLASS}>
                    <th className="px-4 py-3 font-medium">Address</th>
                    <th className="px-4 py-3 text-right font-medium">
                      Balance
                    </th>
                    <th className="px-4 py-3 text-right font-medium">
                      Share of supply
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {visible.map((h) => (
                    <tr
                      key={h.address}
                      className="border-b border-[var(--color-border)] last:border-b-0"
                    >
                      <td className="px-4 py-3 font-mono text-xs">
                        <MonoId value={h.address} />
                      </td>
                      <td className="px-4 py-3 text-right font-mono text-xs">
                        {formatTokenAmount(h.balance, pages.decimals)}
                      </td>
                      <td className="px-4 py-3 text-right font-mono text-xs">
                        {shareOfSupply(h.balance, total).toFixed(2)}%
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <LoadMore
            hasMore={pages.hasMore}
            loading={pages.loadingMore}
            error={pages.moreError}
            onLoadMore={pages.loadMore}
            label="holders"
          />
        </>
      )}
    </section>
  );
}

export function ContractTokensPanel({ contractId }: { contractId: string }) {
  return (
    <div className="space-y-8">
      <TransfersSection contractId={contractId} />
      <HoldersSection contractId={contractId} />
    </div>
  );
}
