"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import type { ContractEvent } from "@/lib/types";
import { getContractEvents } from "@/lib/api";
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

    function startPollingFallback() {
      if (isCancelled) return;
      setStatus("polling");

      async function poll() {
        if (isCancelled) return;
        try {
          if (contractId) {
            const data = await getContractEvents(contractId, { limit: 20 });
            if (!isCancelled && data.events) {
              data.events.forEach((ev) => addEvent(ev));
            }
          }
        } catch {
          // ignore polling errors
        }
      }

      poll();
      fallbackTimerRef.current = setInterval(poll, pollFallbackInterval);
    }

    // Connect via SSE
    if (typeof window === "undefined" || typeof EventSource === "undefined") {
      startPollingFallback();
      return;
    }

    const searchParams = new URLSearchParams();
    if (contractId) {
      searchParams.set("contract_id", contractId);
    }
    const qs = searchParams.toString();
    const url = `${API_URL}/api/v1/stream/events${qs ? "?" + qs : ""}`;

    setStatus("connecting");
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onopen = () => {
      if (isCancelled) return;
      failureCountRef.current = 0;
      setStatus("connected");
      if (fallbackTimerRef.current) {
        clearInterval(fallbackTimerRef.current);
        fallbackTimerRef.current = null;
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

      // If SSE repeatedly errors (e.g. 3 consecutive times), fallback to polling
      if (failureCountRef.current >= 3 && !fallbackTimerRef.current) {
        es.close();
        startPollingFallback();
      } else {
        // EventSource will automatically attempt to reconnect after transient errors
        setStatus("connecting");
      }
    };

    return () => {
      isCancelled = true;
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      if (fallbackTimerRef.current) {
        clearInterval(fallbackTimerRef.current);
        fallbackTimerRef.current = null;
      }
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
