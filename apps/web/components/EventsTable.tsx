"use client";

import { Fragment, useRef, useState } from "react";
import { CopyButton, MonoId } from "@sorolens/ui";
import { useVirtualizer } from "@tanstack/react-virtual";
import { decodeTopic } from "@sorolens/xdr";
import { useLocale } from "@/lib/i18n";
import type { ContractEvent } from "@/lib/types";

interface EventsTableProps {
  events: ContractEvent[];
  loading?: boolean;
  onLoadMore?: () => void;
  hasMore?: boolean;
}

function EventRow({
  event,
  expanded,
  onToggle,
}: {
  event: ContractEvent;
  expanded: boolean;
  onToggle: () => void;
}) {
  const decodedTopics = event.topic_decoded ?? decodeTopic(event.topic_xdr);

  // Fully-decoded event as pretty-printed JSON for pasting into decoders or
  // bug reports (issue #183).
  const decodedEventJson = JSON.stringify(
    { ...event, topic_decoded: decodedTopics },
    null,
    2
  );

  return (
    <tr
      className="cursor-pointer border-b border-[var(--color-border)] transition-colors hover:bg-white/5"
      onClick={onToggle}
    >
      <td className="px-4 py-3 font-mono text-xs">
        <MonoId value={event.tx_hash} headChars={8} tailChars={8} />
      </td>
      <td className="px-4 py-3 text-sm">{event.type}</td>
      <td className="max-w-xs truncate px-4 py-3 font-mono text-xs text-[var(--color-text-secondary)]">
        {String(decodedTopics?.[0] ?? "")}
      </td>
      <td className="max-w-[160px] truncate px-4 py-3 font-mono text-xs text-[var(--color-text-secondary)]">
        {event.value_decoded != null ? String(event.value_decoded) : "-"}
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
      <td className="px-4 py-3 text-right">
        <CopyButton
          text={decodedEventJson}
          label={`Copy JSON for event ${event.id}`}
        />
      </td>
    </tr>
  );
}

function ExpandedEventRow({ event }: { event: ContractEvent }) {
  const { formatDateTime } = useLocale();
  const decodedTopics = event.topic_decoded ?? decodeTopic(event.topic_xdr);
  return (
    <tr className="bg-white/[0.02]">
      <td
        colSpan={7}
        className="border-b border-[var(--color-border)] px-8 py-4"
      >
        <div className="space-y-3">
          <div>
            <div className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
              Decoded Topics
            </div>
            <pre className="max-h-40 overflow-auto rounded bg-black/20 p-3 font-mono text-xs leading-relaxed">
              {JSON.stringify(decodedTopics, null, 2)}
            </pre>
          </div>
          <div>
            <div className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
              Raw Value
            </div>
            <pre className="max-h-40 overflow-auto rounded bg-black/20 p-3 font-mono text-xs leading-relaxed">
              {event.value_decoded != null
                ? JSON.stringify(event.value_decoded, null, 2)
                : event.value_xdr}
            </pre>
          </div>
          <div className="flex gap-4 text-xs text-[var(--color-text-secondary)]">
            <span>Event ID: {event.id}</span>
            <span>Ledger: {event.ledger}</span>
            <span>Time: {formatDateTime(event.ledger_closed_at)}</span>
          </div>
        </div>
      </td>
    </tr>
  );
}

export function EventsTable({
  events,
  loading,
  onLoadMore,
  hasMore,
}: EventsTableProps) {
  const scrollElementRef = useRef<HTMLDivElement>(null);
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());
  const virtualized = events.length > 100;
  const rowVirtualizer = useVirtualizer({
    count: virtualized ? events.length : 0,
    getScrollElement: () => scrollElementRef.current,
    estimateSize: (index) => (expandedIds.has(events[index]?.id) ? 360 : 56),
    getItemKey: (index) => events[index]?.id ?? index,
    initialRect: { width: 1024, height: 560 },
    overscan: 10,
  });

  if (!loading && events.length === 0) {
    return (
      <div className="rounded-lg bg-[var(--color-bg-card)] p-8 text-center text-sm text-[var(--color-text-secondary)]">
        No events found
      </div>
    );
  }

  return (
    <div className="rounded-lg bg-[var(--color-bg-card)]">
      <div
        ref={scrollElementRef}
        data-testid="events-table-scroll"
        className={
          virtualized ? "max-h-[70vh] overflow-auto" : "overflow-x-auto"
        }
      >
        <table className="w-full">
          <thead
            className={
              virtualized
                ? "sticky top-0 z-10 bg-[var(--color-bg-card)]"
                : undefined
            }
          >
            <tr className="border-b border-[var(--color-border)] text-left text-xs font-medium text-[var(--color-text-secondary)]">
              <th className="px-4 py-3">Transaction</th>
              <th className="px-4 py-3">Type</th>
              <th className="px-4 py-3">Topic</th>
              <th className="px-4 py-3">Value</th>
              <th className="px-4 py-3 text-right">Ledger</th>
              <th className="px-4 py-3 text-right">Status</th>
              <th className="px-4 py-3 text-right">Copy</th>
            </tr>
          </thead>
          <tbody>
            {virtualized ? (
              <>
                {(rowVirtualizer.getVirtualItems()[0]?.start ?? 0) > 0 && (
                  <tr aria-hidden="true">
                    <td
                      colSpan={7}
                      style={{
                        height: rowVirtualizer.getVirtualItems()[0].start,
                      }}
                    />
                  </tr>
                )}
                {rowVirtualizer.getVirtualItems().map((row) => {
                  const event = events[row.index];
                  const expanded = expandedIds.has(event.id);
                  return (
                    <Fragment key={event.id}>
                      <EventRow
                        event={event}
                        expanded={expanded}
                        onToggle={() =>
                          setExpandedIds((current) => {
                            const next = new Set(current);
                            if (next.has(event.id)) next.delete(event.id);
                            else next.add(event.id);
                            return next;
                          })
                        }
                      />
                      {expanded && <ExpandedEventRow event={event} />}
                    </Fragment>
                  );
                })}
                {rowVirtualizer.getVirtualItems().length > 0 && (
                  <tr aria-hidden="true">
                    <td
                      colSpan={7}
                      style={{
                        height:
                          rowVirtualizer.getTotalSize() -
                          rowVirtualizer.getVirtualItems().at(-1)!.end,
                      }}
                    />
                  </tr>
                )}
              </>
            ) : (
              events.map((event) => (
                <Fragment key={event.id}>
                  <EventRow
                    event={event}
                    expanded={expandedIds.has(event.id)}
                    onToggle={() =>
                      setExpandedIds((current) => {
                        const next = new Set(current);
                        if (next.has(event.id)) next.delete(event.id);
                        else next.add(event.id);
                        return next;
                      })
                    }
                  />
                  {expandedIds.has(event.id) && (
                    <ExpandedEventRow event={event} />
                  )}
                </Fragment>
              ))
            )}
          </tbody>
        </table>
      </div>
      {hasMore && (
        <div className="border-t border-[var(--color-border)] p-4 text-center">
          <button
            type="button"
            onClick={onLoadMore}
            disabled={loading}
            aria-busy={loading}
            className="rounded-md bg-white/10 px-4 py-2 text-sm font-medium text-[var(--color-text-primary)] transition-colors hover:bg-white/20 disabled:opacity-50"
          >
            {loading ? "Loading..." : "Load More"}
          </button>
        </div>
      )}
    </div>
  );
}
