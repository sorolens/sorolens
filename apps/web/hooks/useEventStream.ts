"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import type { ContractEvent } from "@/lib/types";
import { getContractEvents, listAllEvents } from "@/lib/api";
import { API_URL } from "@/lib/config";

export interface UseEventStreamOptions {
  contractId?: string;
  enabled?: boolean;
  pollFallbackInterval?: number; // fallback polling interval in ms, default 5000
  onEvent?: (event: ContractEvent) => void;
  onAlert?: (alert: unknown) => void;
}

export type StreamStatus =
  "connecting" | "connected" | "polling" | "disconnected";

export const MAX_BUFFERED_EVENTS = 500;

// How many recent events a reconnect catch-up asks for. The buffer de-dupes
// by id, so this only needs to cover the events missed while disconnected.
export const CATCH_UP_LIMIT = 50;

// Delays before re-establishing the SSE connection after an error, in ms.
// The browser's own EventSource reconnect has no way to carry our last event
// id (the server never sends `id:` lines, so there is no Last-Event-ID header
// to echo back), so the hook owns reconnection and passes the id explicitly.
const RECONNECT_DELAYS_MS = [1000, 2000, 4000];

interface EventBuffer {
  events: ContractEvent[];
  droppedCount: number;
}

export function appendStreamEvent(
  buffer: EventBuffer,
  event: ContractEvent
): EventBuffer {
  if (buffer.events.some((existing) => existing.id === event.id)) return buffer;

  const events = [event, ...buffer.events].slice(0, MAX_BUFFERED_EVENTS);
  return {
    events,
    droppedCount:
      buffer.droppedCount +
      (buffer.events.length >= MAX_BUFFERED_EVENTS ? 1 : 0),
  };
}

/**
 * Fetches the newest events so a reconnect can fill the gap left while the
 * stream was down. Works with or without a contract filter; the buffer's
 * id de-duplication means events already seen are dropped rather than
 * duplicated.
 */
async function fetchRecentEvents(
  contractId: string | undefined
): Promise<ContractEvent[]> {
  if (contractId) {
    const data = await getContractEvents(contractId, { limit: CATCH_UP_LIMIT });
    return data.events ?? [];
  }
  const data = await listAllEvents({ limit: CATCH_UP_LIMIT });
  return data.events ?? [];
}

export function useEventStream(
  contractIdOrOptions?: string | UseEventStreamOptions
) {
  const options =
    typeof contractIdOrOptions === "string"
      ? { contractId: contractIdOrOptions }
      : contractIdOrOptions || {};

  const {
    contractId,
    enabled = true,
    pollFallbackInterval = 5000,
    onEvent,
    onAlert,
  } = options;

  const [buffer, setBuffer] = useState<EventBuffer>({
    events: [],
    droppedCount: 0,
  });
  const [status, setStatus] = useState<StreamStatus>("disconnected");
  const [lastEventAt, setLastEventAt] = useState<Date | null>(null);

  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;
  const onAlertRef = useRef(onAlert);
  onAlertRef.current = onAlert;

  const failureCountRef = useRef(0);
  const fallbackTimerRef = useRef<NodeJS.Timeout | null>(null);
  const reconnectTimerRef = useRef<NodeJS.Timeout | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const lastEventIdRef = useRef<string | null>(null);

  const addEvent = useCallback((event: ContractEvent) => {
    setBuffer((prev) => appendStreamEvent(prev, event));
    setLastEventAt(new Date());
    if (onEventRef.current) {
      onEventRef.current(event);
    }
  }, []);

  const clearEvents = useCallback(() => {
    setBuffer({ events: [], droppedCount: 0 });
  }, []);

  useEffect(() => {
    if (!enabled) {
      setStatus("disconnected");
      return;
    }

    let isCancelled = false;

    // A catch-up runs after every successful (re)connection where we already
    // saw events, i.e. only after a reconnect, and asks the REST API for the
    // newest events so the gap while disconnected is filled. Without it,
    // resuming the stream drops everything published while we were offline.
    async function catchUp() {
      try {
        const missed = await fetchRecentEvents(contractId);
        if (isCancelled) return;
        // Newest first; appendStreamEvent de-dupes against the buffer.
        missed.forEach((ev) => addEvent(ev));
      } catch {
        // A failed catch-up is not fatal: the stream itself is still live.
      }
    }

    function stopPollingFallback() {
      if (fallbackTimerRef.current) {
        clearInterval(fallbackTimerRef.current);
        fallbackTimerRef.current = null;
      }
    }

    function startPollingFallback() {
      if (isCancelled) return;
      setStatus("polling");

      async function poll() {
        if (isCancelled) return;
        try {
          // The fallback must work for the global stream too (no
          // contract_id): otherwise the hook silently does nothing when the
          // unfiltered stream falls back to polling.
          const events = await fetchRecentEvents(contractId);
          if (!isCancelled) {
            events.forEach((ev) => addEvent(ev));
          }
        } catch {
          // ignore polling errors
        }
      }

      poll();
      stopPollingFallback();
      fallbackTimerRef.current = setInterval(poll, pollFallbackInterval);
    }

    // Connect via SSE. The connection is recreated by the hook on error
    // (rather than letting EventSource reconnect itself) because the id of
    // the last event received has to travel with the reconnect request — as
    // a query param, since EventSource cannot set request headers — so the
    // stream can resume where it left off instead of from "now".
    function connect() {
      if (isCancelled) return;

      const searchParams = new URLSearchParams();
      if (contractId) {
        searchParams.set("contract_id", contractId);
      }
      if (lastEventIdRef.current) {
        searchParams.set("last_event_id", lastEventIdRef.current);
      }
      const qs = searchParams.toString();
      const url = `${API_URL}/api/v1/stream/events${qs ? "?" + qs : ""}`;

      setStatus("connecting");
      const es = new EventSource(url);
      eventSourceRef.current = es;

      es.onopen = () => {
        if (isCancelled) return;
        const isReconnect = failureCountRef.current > 0;
        failureCountRef.current = 0;
        stopPollingFallback();
        setStatus("connected");
        // Only a reconnect has a gap to fill; the first connection has not
        // missed anything yet.
        if (isReconnect && lastEventIdRef.current) {
          void catchUp();
        }
      };

      es.onmessage = (e) => {
        if (isCancelled) return;
        try {
          const data = JSON.parse(e.data);
          if (data.type === "event" && data.event) {
            addEvent(data.event);
            lastEventIdRef.current = data.event.id;
          } else if (data.type === "alert" && data.alert) {
            if (onAlertRef.current) {
              onAlertRef.current(data.alert);
            }
          }
        } catch {
          // non-JSON message / ping, keep alive
        }
      };

      es.onerror = () => {
        if (isCancelled) return;
        failureCountRef.current += 1;
        es.close();
        if (eventSourceRef.current === es) {
          eventSourceRef.current = null;
        }

        // After repeated failures (e.g. a proxy that refuses SSE), give up
        // on streaming and fall back to polling. Fewer than that: retry the
        // stream with a backoff, resuming from the last event we saw.
        if (failureCountRef.current >= 3) {
          startPollingFallback();
        } else {
          setStatus("connecting");
          const delay =
            RECONNECT_DELAYS_MS[
              Math.min(
                failureCountRef.current - 1,
                RECONNECT_DELAYS_MS.length - 1
              )
            ];
          reconnectTimerRef.current = setTimeout(connect, delay);
        }
      };
    }

    if (typeof window === "undefined" || typeof EventSource === "undefined") {
      startPollingFallback();
    } else {
      connect();
    }

    return () => {
      isCancelled = true;
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      stopPollingFallback();
      setStatus("disconnected");
    };
  }, [contractId, enabled, pollFallbackInterval, addEvent]);

  return {
    events: buffer.events,
    droppedCount: buffer.droppedCount,
    clearEvents,
    status,
    isConnected: status === "connected",
    isPolling: status === "polling",
    lastEventAt,
  };
}
