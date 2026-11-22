"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  ApiError,
  getInvocationTrace,
  resolveLabel,
  searchContracts,
} from "@/lib/api";
import { useNetwork, networkFilter } from "@/lib/network";
import type {
  LabelResolution,
  SearchContract,
  TraceResponse,
} from "@/lib/types";

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="mb-10">
      <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-[var(--color-text-secondary)]">
        {title}
      </h2>
      {children}
    </section>
  );
}

function ErrorMessage({ message }: { message: string }) {
  return (
    <p className="text-sm text-[var(--color-danger)]" role="alert">
      {message}
    </p>
  );
}

function contractMessage(error: unknown): string {
  return error instanceof ApiError
    ? error.message
    : "Search is unavailable right now.";
}

function isTxHash(query: string): boolean {
  return /^[0-9a-fA-F]{64}$/.test(query.trim());
}

function ContractResult({ contract }: { contract: SearchContract }) {
  return (
    <li>
      <Link
        href={`/contracts/${contract.id}`}
        className="flex items-center justify-between gap-4 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-3 transition-colors hover:border-[var(--color-text-secondary)]"
      >
        <span className="min-w-0">
          <span className="block truncate font-medium text-[var(--color-text-primary)]">
            {contract.label || "Unnamed Contract"}
          </span>
          <span
            className="block truncate font-mono text-xs text-[var(--color-text-secondary)]"
            title={contract.id}
          >
            {contract.id}
          </span>
        </span>
        <span className="shrink-0 rounded-full border border-[var(--color-border)] px-2 py-1 text-xs capitalize text-[var(--color-text-secondary)]">
          {contract.network}
        </span>
      </Link>
    </li>
  );
}

function SearchResults({ query }: { query: string }) {
  const { network } = useNetwork();
  const networkParam = networkFilter(network);

  const [contracts, setContracts] = useState<SearchContract[]>([]);
  const [contractsLoading, setContractsLoading] = useState(false);
  const [contractsError, setContractsError] = useState<string | null>(null);

  const [label, setLabel] = useState<LabelResolution | null>(null);
  const [labelError, setLabelError] = useState<string | null>(null);

  const [trace, setTrace] = useState<TraceResponse | null>(null);
  const [traceError, setTraceError] = useState<string | null>(null);

  useEffect(() => {
    if (!query) {
      setContracts([]);
      setContractsError(null);
      setLabel(null);
      setLabelError(null);
      setTrace(null);
      setTraceError(null);
      return;
    }

    let active = true;
    setContractsLoading(true);
    setContractsError(null);
    searchContracts(query, { limit: 20, network: networkParam })
      .then((data) => {
        if (!active) return;
        const items = Array.isArray(data.items) ? data.items : [];
        setContracts(
          networkParam ? items.filter((c) => c.network === networkParam) : items
        );
      })
      .catch((error) => {
        if (!active) return;
        setContracts([]);
        setContractsError(contractMessage(error));
      })
      .finally(() => {
        if (active) setContractsLoading(false);
      });

    setLabel(null);
    setLabelError(null);
    resolveLabel(query)
      .then((resolution) => {
        if (active) setLabel(resolution);
      })
      .catch((error) => {
        if (!active) return;
        // A label miss is a normal outcome, not a failure worth surfacing.
        setLabelError(
          error instanceof ApiError && error.status === 404
            ? null
            : contractMessage(error)
        );
      });

    setTrace(null);
    setTraceError(null);
    if (isTxHash(query)) {
      getInvocationTrace(query.trim())
        .then((result) => {
          if (active) setTrace(result);
        })
        .catch((error) => {
          if (!active) return;
          setTraceError(contractMessage(error));
        });
    }

    return () => {
      active = false;
    };
  }, [query, networkParam]);

  return (
    <div>
      <Section title="Contracts">
        {contractsError && <ErrorMessage message={contractsError} />}
        {!contractsError && contractsLoading && (
          <p
            role="status"
            aria-busy="true"
            className="text-sm text-[var(--color-text-secondary)]"
          >
            Searching...
          </p>
        )}
        {!contractsError && !contractsLoading && contracts.length === 0 && (
          <p className="text-sm text-[var(--color-text-secondary)]">
            No contracts matched &ldquo;{query}&rdquo;.
          </p>
        )}
        {contracts.length > 0 && (
          <ul className="space-y-2">
            {contracts.map((contract) => (
              <ContractResult key={contract.id} contract={contract} />
            ))}
          </ul>
        )}
      </Section>

      <Section title="Labels">
        {labelError && <ErrorMessage message={labelError} />}
        {label ? (
          label.scope === "raw" && !label.value.startsWith("C") ? (
            <div className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-3 text-sm">
              <span className="font-medium text-[var(--color-text-primary)]">
                {label.label}
              </span>
              <span className="ml-2 text-[var(--color-text-secondary)]">
                {label.scope}
              </span>
              <span className="mt-1 block truncate font-mono text-xs text-[var(--color-text-secondary)]">
                {label.value}
              </span>
            </div>
          ) : (
            <Link
              href={`/contracts/${label.value}`}
              className="block rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-3 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-text-secondary)]"
            >
              <span className="font-medium">{label.label}</span>
              <span className="ml-2 text-[var(--color-text-secondary)]">
                {label.scope}
              </span>
              <span className="mt-1 block truncate font-mono text-xs text-[var(--color-text-secondary)]">
                {label.value}
              </span>
            </Link>
          )
        ) : (
          !labelError && (
            <p className="text-sm text-[var(--color-text-secondary)]">
              No label resolved for &ldquo;{query}&rdquo;.
            </p>
          )
        )}
      </Section>

      <Section title="Transactions">
        {!isTxHash(query) && (
          <p className="text-sm text-[var(--color-text-secondary)]">
            Paste a 64-character transaction hash to look up its invocation
            trace.
          </p>
        )}
        {traceError && <ErrorMessage message={traceError} />}
        {trace && (
          <div className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-3 text-sm">
            <span className="block break-all font-mono text-[var(--color-text-primary)]">
              {trace.tx_hash}
            </span>
            <span className="mt-1 block text-[var(--color-text-secondary)]">
              Ledger {trace.ledger}
              {trace.network ? ` · ${trace.network}` : ""}
              {trace.status ? ` · ${trace.status}` : ""} · {trace.edge_count}{" "}
              cross-contract {trace.edge_count === 1 ? "call" : "calls"}
            </span>
          </div>
        )}
        {isTxHash(query) && !trace && !traceError && (
          <p className="text-sm text-[var(--color-text-secondary)]">
            Looking up transaction...
          </p>
        )}
      </Section>
    </div>
  );
}

function SearchPageInner() {
  const params = useSearchParams();
  const query = (params.get("q") || "").trim();
  const [value, setValue] = useState(query);

  useEffect(() => {
    setValue(query);
  }, [query]);

  return (
    <div className="mx-auto max-w-3xl">
      <h1 className="mb-2 text-2xl font-bold text-[var(--color-text-primary)]">
        Search
      </h1>
      <p className="mb-6 text-sm text-[var(--color-text-secondary)]">
        Contracts, labels and transactions matching your query.
      </p>

      <form
        action="/search"
        method="get"
        className="mb-8 flex gap-2"
        role="search"
      >
        <input
          type="search"
          name="q"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          placeholder="Search by contract ID, label or transaction hash..."
          aria-label="Search query"
          className="flex-1 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2 text-sm text-[var(--color-text-primary)] outline-none placeholder:text-[var(--color-text-secondary)] focus:border-[var(--color-text-secondary)]"
        />
        <button
          type="submit"
          className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-4 py-2 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-text-secondary)]"
        >
          Search
        </button>
      </form>

      {query ? (
        <SearchResults query={query} />
      ) : (
        <p className="text-sm text-[var(--color-text-secondary)]">
          Enter a query to search contracts, labels and transactions.
        </p>
      )}
    </div>
  );
}

export default function SearchPage() {
  return (
    <Suspense
      fallback={
        <p
          role="status"
          aria-busy="true"
          className="text-sm text-[var(--color-text-secondary)]"
        >
          Loading...
        </p>
      }
    >
      <SearchPageInner />
    </Suspense>
  );
}
