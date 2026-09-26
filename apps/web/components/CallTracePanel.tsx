"use client";

import { type FormEvent, useEffect, useRef, useState } from "react";
import { MonoId } from "@sorolens/ui";
import { ApiError, getInvocationTrace } from "@/lib/api";
import type { TraceResponse } from "@/lib/types";
import { TraceFlameGraph } from "./TraceFlameGraph";

const TX_HASH_RE = /^[0-9a-f]{64}$/i;

interface CallTracePanelProps {
  /**
   * A transaction hash to load on mount and whenever it changes. The contract
   * detail page passes its newest event's tx hash so the most recent trace is
   * shown without any typing.
   */
  initialTxHash?: string;
}

/**
 * Loads and renders the cross-contract call tree of a transaction
 * (`GET /api/v1/invocations/{tx_hash}/trace`).
 */
export function CallTracePanel({ initialTxHash = "" }: CallTracePanelProps) {
  const [input, setInput] = useState(initialTxHash);
  const [trace, setTrace] = useState<TraceResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Auto-load the prefilled transaction exactly once. The page derives it from
  // its newest event, which keeps changing as the live stream delivers events;
  // re-loading on every change would clobber a trace the user searched for.
  const autoLoaded = useRef(false);
  useEffect(() => {
    if (!autoLoaded.current && initialTxHash) {
      autoLoaded.current = true;
      setInput(initialTxHash);
      void loadTrace(initialTxHash);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialTxHash]);

  async function loadTrace(hash: string) {
    const normalized = hash.trim().toLowerCase();
    if (!TX_HASH_RE.test(normalized)) {
      setError("Enter a 64-character transaction hash.");
      setTrace(null);
      return;
    }

    setLoading(true);
    setError(null);
    try {
      const data = await getInvocationTrace(normalized);
      setTrace(data);
    } catch (err) {
      setTrace(null);
      if (err instanceof ApiError && err.status === 404) {
        setError("No invocation found for that transaction hash.");
      } else {
        setError("Could not load the call trace. Please try again.");
      }
    } finally {
      setLoading(false);
    }
  }

  function handleSubmit(event: FormEvent) {
    event.preventDefault();
    void loadTrace(input);
  }

  return (
    <div className="space-y-4">
      <form onSubmit={handleSubmit} className="flex flex-wrap items-center gap-2">
        <input
          id="trace-tx-hash"
          aria-label="Transaction hash"
          value={input}
          onChange={(event) => setInput(event.target.value)}
          placeholder="Transaction hash (64 hex characters)"
          spellCheck={false}
          className="min-w-0 flex-1 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-2 font-mono text-xs text-[var(--color-text-primary)] outline-none focus:border-[var(--color-accent)]"
        />
        <button
          type="submit"
          disabled={loading}
          className="rounded-md bg-white/10 px-4 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:bg-white/20 disabled:opacity-50"
        >
          {loading ? "Loading..." : "Load trace"}
        </button>
      </form>

      {error && (
        <div
          role="alert"
          className="rounded-lg border border-[var(--color-danger)] bg-[var(--color-bg-card)] px-4 py-3 text-sm text-[var(--color-danger)]"
        >
          {error}
        </div>
      )}

      {loading && !trace && (
        <div className="rounded-lg bg-[var(--color-bg-card)] p-8 text-center text-sm text-[var(--color-text-secondary)]">
          Loading call trace…
        </div>
      )}

      {trace && (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-[var(--color-text-secondary)]">
            <span className="font-mono">
              <MonoId value={trace.tx_hash} headChars={8} tailChars={8} />
            </span>
            {trace.status && (
              <span
                className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${
                  trace.status === "SUCCESS"
                    ? "bg-green-900/40 text-green-400"
                    : "bg-red-900/40 text-red-400"
                }`}
              >
                {trace.status}
              </span>
            )}
            {trace.network && <span>Network: {trace.network}</span>}
            <span>Ledger: {trace.ledger.toLocaleString()}</span>
            <span>
              {trace.edge_count} cross-contract call
              {trace.edge_count === 1 ? "" : "s"}
            </span>
          </div>
          <TraceFlameGraph
            root={trace.root}
            hasEdges={trace.has_edges}
            truncated={trace.truncated}
          />
        </div>
      )}
    </div>
  );
}
