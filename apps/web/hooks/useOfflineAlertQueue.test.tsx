/**
 * useOfflineAlertQueue.test.tsx — tests for the offline alert queue hook.
 *
 * Mocks alertQueue.ts so we don't depend on a real IndexedDB in the hook
 * tests — that is tested separately in lib/alertQueue.test.ts.  Here we only
 * test the hook's state management (pending list, isOffline, dismiss/all).
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import type { QueuedAlert } from "@/lib/alertQueue";
import type { ContractAlert } from "@/lib/types";

// ------------------------------------------------------------------
// Mock the alertQueue module so no real IDB calls are made here.
// The IDB interface itself is exercised by lib/alertQueue.test.ts.
// ------------------------------------------------------------------
const mockAlerts: QueuedAlert[] = [];

vi.mock("@/lib/alertQueue", () => ({
  enqueueAlert: vi.fn(
    async (raw: Omit<QueuedAlert, "enqueuedAt" | "dismissed">) => {
      // Add to the in-memory list only if not duplicate
      if (!mockAlerts.find((a) => a.id === raw.id)) {
        mockAlerts.push({
          ...raw,
          enqueuedAt: new Date().toISOString(),
          dismissed: false,
        });
      }
    }
  ),
  getPendingAlerts: vi.fn(async () => mockAlerts.filter((a) => !a.dismissed)),
  getPendingCount: vi.fn(
    async () => mockAlerts.filter((a) => !a.dismissed).length
  ),
  dismissAlert: vi.fn(async (id: string) => {
    const a = mockAlerts.find((x) => x.id === id);
    if (a) a.dismissed = true;
  }),
  dismissAll: vi.fn(async () => {
    mockAlerts.forEach((a) => {
      a.dismissed = true;
    });
  }),
  deleteAlert: vi.fn(async (id: string) => {
    const idx = mockAlerts.findIndex((x) => x.id === id);
    if (idx !== -1) mockAlerts.splice(idx, 1);
  }),
  purgeDismissed: vi.fn(async () => {
    const toPurge = mockAlerts
      .map((a, i) => (a.dismissed ? i : -1))
      .filter((i) => i !== -1)
      .reverse();
    toPurge.forEach((i) => mockAlerts.splice(i, 1));
  }),
  _closeForTests: vi.fn(async () => {}),
}));

beforeEach(() => {
  // Reset the in-memory mock store
  mockAlerts.splice(0, mockAlerts.length);
  vi.clearAllMocks();

  // Start online
  Object.defineProperty(navigator, "onLine", {
    value: true,
    configurable: true,
    writable: true,
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useOfflineAlertQueue", () => {
  it("starts with an empty pending list", async () => {
    const { useOfflineAlertQueue } =
      await import("@/hooks/useOfflineAlertQueue");
    const { result } = renderHook(() => useOfflineAlertQueue());
    await waitFor(() => expect(result.current.pending).toHaveLength(0));
  });

  it("queueAlert persists an alert and it appears in pending", async () => {
    const { useOfflineAlertQueue } =
      await import("@/hooks/useOfflineAlertQueue");
    const { result } = renderHook(() => useOfflineAlertQueue());

    const alert: ContractAlert = {
      contract_id: "CABC123456789",
      severity: "Critical",
      message: "Storage expiry imminent",
      ledger: 99999,
      tx_hash: "abcdef",
      timestamp: new Date().toISOString(),
    };

    await act(async () => {
      await result.current.queueAlert(alert);
    });

    await waitFor(() =>
      expect(result.current.pending.length).toBeGreaterThan(0)
    );
    expect(result.current.pending[0].contractId).toBe("CABC123456789");
    expect(result.current.pending[0].severity).toBe("Critical");
  });

  it("dismiss removes an alert from pending", async () => {
    const { useOfflineAlertQueue } =
      await import("@/hooks/useOfflineAlertQueue");
    const { result } = renderHook(() => useOfflineAlertQueue());

    await act(async () => {
      await result.current.queueAlert({
        contract_id: "CDISMISS",
        severity: "Warning",
        message: "Will be dismissed",
        ledger: 1,
        tx_hash: "tx1",
        timestamp: new Date().toISOString(),
      });
    });

    await waitFor(() =>
      expect(result.current.pending.length).toBeGreaterThan(0)
    );

    const id = result.current.pending[0].id;

    await act(async () => {
      await result.current.dismiss(id);
    });

    await waitFor(() =>
      expect(result.current.pending.find((a) => a.id === id)).toBeUndefined()
    );
  });

  it("dismissAll empties the pending list", async () => {
    const { useOfflineAlertQueue } =
      await import("@/hooks/useOfflineAlertQueue");
    const { result } = renderHook(() => useOfflineAlertQueue());

    for (let i = 0; i < 3; i++) {
      await act(async () => {
        await result.current.queueAlert({
          contract_id: `C${i}`,
          severity: "Info",
          message: `Alert ${i}`,
          ledger: i,
          tx_hash: `tx${i}`,
          timestamp: new Date().toISOString(),
        });
      });
    }

    await waitFor(() =>
      expect(result.current.pending.length).toBeGreaterThan(0)
    );

    await act(async () => {
      await result.current.dismissAll();
    });

    await waitFor(() => expect(result.current.pending).toHaveLength(0));
  });

  it("isOffline is false when navigator.onLine is true", async () => {
    const { useOfflineAlertQueue } =
      await import("@/hooks/useOfflineAlertQueue");
    const { result } = renderHook(() => useOfflineAlertQueue());
    await waitFor(() => expect(result.current.isOffline).toBe(false));
  });

  it("isOffline is true when navigator.onLine is false", async () => {
    Object.defineProperty(navigator, "onLine", {
      value: false,
      configurable: true,
      writable: true,
    });
    const { useOfflineAlertQueue } =
      await import("@/hooks/useOfflineAlertQueue");
    const { result } = renderHook(() => useOfflineAlertQueue());
    await waitFor(() => expect(result.current.isOffline).toBe(true));
  });
});
