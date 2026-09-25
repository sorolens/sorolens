"use client";

import { useEffect, useRef, useState } from "react";
import type { ContractEvent } from "@/lib/types";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
// ws:// or wss:// derived from the HTTP API URL.
const WS_BASE = API_URL.replace(/^http/, "ws");

const SUBSCRIBE_ID = "main";
const RECONNECT_DELAY_MS = 3000;

export interface UseSubscriptionOptions {
  /** Only receive events/alerts published for this contract; omit for all. */
  contractId?: string;
  /** Set false to close the socket and stop reconnecting. Default true. */
  enabled?: boolean;
  onEvent?: (event: ContractEvent) => void;
  onAlert?: (alert: unknown) => void;
}

export type SubscriptionStatus = "connecting" | "connected" | "disconnected";

interface SubscriptionServerMessage {
  op?: "subscribed" | "unsubscribed" | "error";
  id?: string;
  type?: "connected" | "event" | "alert" | "heartbeat";
  contract_id?: string;
  event?: ContractEvent;
  alert?: unknown;
  error?: string;
}

/**
 * useSubscription connects to the API's WebSocket subscription endpoint
 * (GET /api/v1/subscribe, issue #126) and receives pushed events/alerts.
 *
 * The filter changes live: when `contractId` changes the hook re-sends the
 * subscribe frame with the same id, which the server treats as a filter
 * replacement — no reconnect needed.
 */
export function useSubscription(options: UseSubscriptionOptions = {}) {
  const { contractId, enabled = true, onEvent, onAlert } = options;

  const [status, setStatus] = useState<SubscriptionStatus>("disconnected");
  const [lastEventAt, setLastEventAt] = useState<Date | null>(null);

  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;
  const onAlertRef = useRef(onAlert);
  onAlertRef.current = onAlert;

  // Desired filter lives in a ref so (re)connects always pick up the latest
  // value without re-running the connection effect.
  const filterRef = useRef(contractId);
  filterRef.current = contractId;

  const wsRef = useRef<WebSocket | null>(null);
  const subscribedRef = useRef(false);

  function sendSubscribe(socket: WebSocket) {
    const filter = filterRef.current
      ? { contract_id: filterRef.current }
      : {};
    socket.send(
      JSON.stringify({ op: "subscribe", id: SUBSCRIBE_ID, filter }),
    );
  }

  useEffect(() => {
    if (!enabled || typeof window === "undefined") {
      setStatus("disconnected");
      return;
    }

    let cancelled = false;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    subscribedRef.current = false;

    function connect() {
      if (cancelled) return;
      setStatus("connecting");
      const socket = new WebSocket(`${WS_BASE}/api/v1/subscribe`);
      wsRef.current = socket;

      socket.onmessage = (e) => {
        if (cancelled) return;
        let msg: SubscriptionServerMessage;
        try {
          msg = JSON.parse(e.data as string);
        } catch {
          return; // non-JSON frame, ignore
        }
        if (msg.type === "connected") {
          subscribedRef.current = true;
          sendSubscribe(socket);
          setStatus("connected");
        } else if (msg.type === "event" && msg.event) {
          setLastEventAt(new Date());
          if (onEventRef.current) onEventRef.current(msg.event);
        } else if (msg.type === "alert") {
          setLastEventAt(new Date());
          if (onAlertRef.current) onAlertRef.current(msg.alert);
        } else if (msg.op === "error") {
          console.warn("useSubscription: server error:", msg.error);
        }
        // "subscribed"/"unsubscribed"/"heartbeat" need no client action;
        // protocol-level pings are answered by the browser automatically.
      };

      socket.onclose = () => {
        if (cancelled) return;
        subscribedRef.current = false;
        setStatus("disconnected");
        reconnectTimer = setTimeout(connect, RECONNECT_DELAY_MS);
      };

      socket.onerror = () => {
        // onclose always fires after onerror; reconnection is handled there.
      };
    }

    connect();

    return () => {
      cancelled = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      const socket = wsRef.current;
      if (
        socket &&
        (socket.readyState === WebSocket.OPEN ||
          socket.readyState === WebSocket.CONNECTING)
      ) {
        socket.onclose = null; // no reconnect attempt during teardown
        socket.close(1000, "hook unmounted");
      }
      wsRef.current = null;
      setStatus("disconnected");
    };
  }, [enabled]);

  // Live filter change: once subscribed, re-send the subscribe frame with the
  // same id on contractId change; the server replaces the filter in place.
  const firstFilterRun = useRef(true);
  useEffect(() => {
    if (firstFilterRun.current) {
      firstFilterRun.current = false;
      return;
    }
    const socket = wsRef.current;
    if (
      !enabled ||
      !socket ||
      !subscribedRef.current ||
      socket.readyState !== WebSocket.OPEN
    ) {
      return; // still connecting/disconnected: connect() uses filterRef
    }
    try {
      sendSubscribe(socket);
    } catch {
      // socket closing; onclose will reconnect with the new filter
    }
  }, [contractId, enabled]);

  return {
    status,
    isConnected: status === "connected",
    lastEventAt,
  };
}
