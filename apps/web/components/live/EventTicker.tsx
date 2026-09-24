"use client";

/**
 * Event ticker for the /live dashboard: the newest events across every tracked
 * contract.
 *
 * Rows are keyed by event id and are never re-created by a poll that does not
 * bring new data, so the list updates in place rather than flashing. Events
 * that arrived on the most recent poll are highlighted briefly so a viewer
 * watching a wall monitor can see activity land.
 */

import { MonoId } from "@sorolens/ui";
import type { ContractEvent } from "@/lib/types";

export interface EventTickerProps {
  events: ContractEvent[];
  /** Ids that arrived on the latest poll. */
  newIds: Set<string>;
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "--:--:--";
  return d.toLocaleTimeString(undefined, { hour12: false });
}

/** First decoded topic, which by Soroban convention names the event. */
function topicName(topic: unknown[] | null): string {
  if (!topic || topic.length === 0) return "event";
  const head = topic[0];
  if (typeof head === "string") return head;
  if (head === null) return "event";
  return JSON.stringify(head);
}

export function EventTicker({ events, newIds }: EventTickerProps) {
  if (events.length === 0) {
    return (
      <p
        data-testid="ticker-empty"
        className="px-4 py-8 text-center text-sm text-[var(--color-text-secondary)]"
      >
        No events yet. The indexer polls the network every 5 minutes, so this
        fills in shortly after a contract emits.
      </p>
    );
  }

  return (
    <ul data-testid="ticker" className="divide-y divide-[var(--color-border)]">
      {events.map((e) => {
        const isNew = newIds.has(e.id);
        return (
          <li
            key={e.id}
            data-testid="ticker-row"
            data-event-id={e.id}
            data-is-new={isNew ? "true" : "false"}
            className={[
              "flex items-center gap-3 px-4 py-2 font-mono text-xs transition-colors duration-700",
              isNew
                ? "bg-[var(--color-accent)]/15 text-[var(--color-text-primary)]"
                : "text-[var(--color-text-secondary)]",
            ].join(" ")}
          >
            <span className="w-20 shrink-0 tabular-nums">
              {formatTime(e.ledger_closed_at)}
            </span>
            <span
              className={[
                "w-28 shrink-0 truncate",
                e.in_successful_call
                  ? "text-[var(--color-text-primary)]"
                  : "text-[var(--color-danger)]",
              ].join(" ")}
              title={topicName(e.topic_decoded)}
            >
              {topicName(e.topic_decoded)}
            </span>
            <span className="shrink-0">
              <MonoId value={e.contract_id} headChars={6} tailChars={4} />
            </span>
            <span className="ml-auto shrink-0 tabular-nums">
              #{e.ledger}
            </span>
          </li>
        );
      })}
    </ul>
  );
}
