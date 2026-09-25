"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { EventsTable } from "@/components/EventsTable";
import { StatCard } from "@/components/StatCard";
import { useEventStream } from "@/hooks/useEventStream";
import { truncateMiddle } from "@/lib/format";
import type { ContractEvent } from "@/lib/types";

export default function EventsPage() {
  const [contractIdFilter, setContractIdFilter] = useState("");
  const [activeFilter, setActiveFilter] = useState("");

  const { events, setEvents, status, isConnected, isPolling, lastEventAt } =
    useEventStream({
      contractId: activeFilter || undefined,
    });

  const handleApplyFilter = (e: React.FormEvent) => {
    e.preventDefault();
    setActiveFilter(contractIdFilter.trim());
    setEvents([]);
  };

  const handleClearFilter = () => {
    setContractIdFilter("");
    setActiveFilter("");
    setEvents([]);
  };

  return (
    <div className="space-y-8">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-3xl font-bold tracking-tight">Live Events</h1>
            <span
              className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ${
                isConnected
                  ? "bg-green-900/40 text-green-400"
                  : isPolling
                    ? "bg-amber-900/40 text-amber-400"
                    : "bg-blue-900/40 text-blue-400"
              }`}
            >
              <span
                className={`h-1.5 w-1.5 rounded-full ${
                  isConnected
                    ? "bg-green-400 animate-pulse"
                    : isPolling
                      ? "bg-amber-400"
                      : "bg-blue-400 animate-ping"
                }`}
              />
              {isConnected
                ? "Live (SSE)"
                : isPolling
                  ? "Live (Polling Fallback)"
                  : "Connecting..."}
            </span>
          </div>
          <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
            Real-time Server-Sent Events stream as contract events are indexed
            on-chain.{" "}
            <Link
              href="/events"
              className="text-[var(--color-accent)] hover:underline"
            >
              Browse indexed history
            </Link>
          </p>
        </div>

        <form onSubmit={handleApplyFilter} className="flex items-center gap-2">
          <input
            type="text"
            placeholder="Filter by Contract ID..."
            value={contractIdFilter}
            onChange={(e) => setContractIdFilter(e.target.value)}
            className="w-64 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-3 py-1.5 text-xs font-mono text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]"
          />
          <button
            type="submit"
            className="rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-xs font-medium text-[var(--color-bg-page)] transition-opacity hover:opacity-90"
          >
            Filter
          </button>
          {activeFilter && (
            <button
              type="button"
              onClick={handleClearFilter}
              className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-xs font-medium text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
            >
              Clear
            </button>
          )}
        </form>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard
          label="Streamed Events"
          value={events.length.toLocaleString()}
          subtext={
            activeFilter
              ? `For ${truncateMiddle(activeFilter)}`
              : "Across all tracked contracts"
          }
        />
        <StatCard
          label="Stream Connection"
          value={
            isConnected ? "Connected" : isPolling ? "Polling" : "Connecting"
          }
          subtext={
            isConnected
              ? "Server-Sent Events active"
              : "HTTP polling fallback active"
          }
        />
        <StatCard
          label="Last Event Received"
          value={
            lastEventAt
              ? lastEventAt.toLocaleTimeString()
              : "Waiting for events..."
          }
          subtext={
            lastEventAt
              ? lastEventAt.toLocaleDateString()
              : "New events appear in real-time"
          }
        />
      </div>

      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold">Incoming Event Feed</h2>
          {events.length > 0 && (
            <button
              type="button"
              onClick={() => setEvents([])}
              className="text-xs text-[var(--color-text-secondary)] underline hover:text-[var(--color-text-primary)]"
            >
              Clear feed
            </button>
          )}
        </div>

        <EventsTable events={events} />
      </section>
    </div>
  );
}
