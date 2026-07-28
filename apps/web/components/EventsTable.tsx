"use client";

import { useState } from "react";
import { MonoId } from "@sorolens/ui";
import { decodeTopic } from "@sorolens/xdr";
import type { ContractEvent } from "@/lib/types";

interface EventsTableProps {
  events: ContractEvent[];
  loading?: boolean;
  onLoadMore?: () => void;
  hasMore?: boolean;
}

function EventRow({ event }: { event: ContractEvent }) {
  const [expanded, setExpanded] = useState(false);

  const decodedTopics = event.topic_decoded ?? decodeTopic(event.topic_xdr);

  return (
    <>
      <tr
        className="cursor-pointer border-b border-[var(--color-border)] transition-colors hover:bg-white/5"
        onClick={() => setExpanded(!expanded)}
      >
        <td className="px-4 py-3 font-mono text-xs">
          <MonoId value={event.tx_hash} headChars={8} tailChars={8} />
        </td>
        <td className="px-4 py-3 text-sm">{event.type}</td>
        <td className="max-w-xs truncate px-4 py-3 font-mono text-xs text-[var(--color-text-secondary)]">
          {String(decodedTopics?.[0] ?? "")}
        </td>
        <td className="max-w-[160px] truncate px-4 py-3 font-mono text-xs text-[var(--color-text-secondary)]">
          {event.value_decoded != null
            ? String(event.value_decoded)
            : "-"}
        </td>
        <td className="px-4 py-3 text-right font-mono text-xs text-[var(--color-text-secondary)]">
          {event.ledger}
        </td>
        <td className="px-4 py-3 text-right text-xs">
          <span
            className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${
              event.in_successful_call
                ? "bg-green-900/40 text-green-400"
                : "bg-red-900/40 text-red-400"
            }`}
          >
            {event.in_successful_call ? "OK" : "FAIL"}
          </span>
        </td>
      </tr>
      {expanded && (
        <tr className="bg-white/[0.02]">
          <td colSpan={6} className="border-b border-[var(--color-border)] px-8 py-4">
            <div className="space-y-3">
              <div>
                <div className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
                  Decoded Topics
                </div>
                <pre className="overflow-x-auto rounded bg-black/20 p-3 font-mono text-xs leading-relaxed">
                  {JSON.stringify(decodedTopics, null, 2)}
                </pre>
              </div>
              <div>
                <div className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
                  Raw Value
                </div>
                <pre className="overflow-x-auto rounded bg-black/20 p-3 font-mono text-xs leading-relaxed">
                  {event.value_decoded != null
                    ? JSON.stringify(event.value_decoded, null, 2)
                    : event.value_xdr}
                </pre>
              </div>
              <div className="flex gap-4 text-xs text-[var(--color-text-secondary)]">
                <span>Event ID: {event.id}</span>
                <span>Ledger: {event.ledger}</span>
                <span>
                  Time:{" "}
                  {new Date(event.ledger_closed_at).toLocaleString()}
                </span>
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  );
}

export function EventsTable({
  events,
  loading,
  onLoadMore,
  hasMore,
}: EventsTableProps) {
  if (!loading && events.length === 0) {
    return (
      <div className="rounded-lg bg-[var(--color-bg-card)] p-8 text-center text-sm text-[var(--color-text-secondary)]">
        No events found
      </div>
    );
  }

  return (
    <div className="rounded-lg bg-[var(--color-bg-card)]">
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="border-b border-[var(--color-border)] text-left text-xs font-medium text-[var(--color-text-secondary)]">
              <th className="px-4 py-3">Transaction</th>
              <th className="px-4 py-3">Type</th>
              <th className="px-4 py-3">Topic</th>
              <th className="px-4 py-3">Value</th>
              <th className="px-4 py-3 text-right">Ledger</th>
              <th className="px-4 py-3 text-right">Status</th>
            </tr>
          </thead>
          <tbody>
            {events.map((event) => (
              <EventRow key={event.id} event={event} />
            ))}
          </tbody>
        </table>
      </div>
      {hasMore && (
        <div className="border-t border-[var(--color-border)] p-4 text-center">
          <button
            type="button"
            onClick={onLoadMore}
            disabled={loading}
            className="rounded-md bg-white/10 px-4 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:bg-white/20 disabled:opacity-50"
          >
            {loading ? "Loading..." : "Load More"}
          </button>
        </div>
      )}
    </div>
  );
}
