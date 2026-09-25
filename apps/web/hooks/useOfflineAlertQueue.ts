/**
 * useOfflineAlertQueue — offline alert queue hook (issue #275).
 *
 * Drains new alerts from `useEventStream`'s `onAlert` callback into the
 * IndexedDB queue while offline, and exposes the pending list so the UI
 * can render the OfflineAlertBanner.
 */
"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  enqueueAlert,
  getPendingAlerts,
  dismissAlert,
  dismissAll,
  type QueuedAlert,
} from "@/lib/alertQueue";
import type { ContractAlert } from "@/lib/types";

/** Milliseconds between queue refreshes when online. */
const POLL_INTERVAL_MS = 5_000;

export function useOfflineAlertQueue() {
  const [pending, setPending] = useState<QueuedAlert[]>([]);
  const [isOffline, setIsOffline] = useState<boolean>(
    typeof navigator !== "undefined" ? !navigator.onLine : false
  );
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Keep isOffline in sync with the browser's network state
  useEffect(() => {
    const handleOnline = () => setIsOffline(false);
    const handleOffline = () => setIsOffline(true);
    window.addEventListener("online", handleOnline);
    window.addEventListener("offline", handleOffline);
    return () => {
      window.removeEventListener("online", handleOnline);
      window.removeEventListener("offline", handleOffline);
    };
  }, []);

  // Refresh queue on mount and on an interval
  const refresh = useCallback(async () => {
    const items = await getPendingAlerts();
    setPending(items);
  }, []);

  useEffect(() => {
    void refresh();
    timerRef.current = setInterval(refresh, POLL_INTERVAL_MS);
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [refresh]);

  /**
   * Enqueue an alert from `useEventStream`'s onAlert callback.
   * When offline the alert will surface in the banner; online it still queues
   * and can be reviewed in the panel.
   */
  const queueAlert = useCallback(
    async (raw: unknown) => {
      const alert = raw as ContractAlert;
      const id =
        [alert.contract_id, alert.ledger, alert.timestamp].join(":") +
        Math.random().toString(36).slice(2, 6);

      await enqueueAlert({
        id,
        contractId: alert.contract_id,
        severity: alert.severity as QueuedAlert["severity"],
        message: alert.message,
        timestamp: alert.timestamp,
      });
      void refresh();
    },
    [refresh]
  );

  const dismiss = useCallback(
    async (id: string) => {
      await dismissAlert(id);
      void refresh();
    },
    [refresh]
  );

  const dismissAllPending = useCallback(async () => {
    await dismissAll();
    void refresh();
  }, [refresh]);

  return {
    pending,
    isOffline,
    queueAlert,
    dismiss,
    dismissAll: dismissAllPending,
  };
}
