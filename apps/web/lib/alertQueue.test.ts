/**
 * alertQueue.test.ts — unit tests for the IndexedDB-backed offline alert queue
 *
 * Uses fake-indexeddb (installed via vitest.setup.ts) which provides a real
 * IDB implementation in the jsdom environment.  Between tests, we close the
 * open database connection and delete the named database so each test starts
 * with an empty store.
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import {
  enqueueAlert,
  getPendingAlerts,
  getPendingCount,
  dismissAlert,
  dismissAll,
  deleteAlert,
  purgeDismissed,
  _closeForTests,
} from "@/lib/alertQueue";

// Close and reset the IDB connection before every test so data does not bleed.
// fake-indexeddb persists in-memory across tests within the same file, so we
// need to explicitly close the previous connection before deleting the DB.
beforeEach(async () => {
  await _closeForTests();
  // Delete the named DB so the next openDB() starts fresh
  await new Promise<void>((resolve) => {
    const req = indexedDB.deleteDatabase("sorolens-pwa");
    req.onsuccess = () => resolve();
    req.onerror = () => resolve();
    req.onblocked = () => resolve();
  });
});

afterEach(async () => {
  await _closeForTests();
});

describe("alertQueue — enqueueAlert / getPendingAlerts", () => {
  it("enqueues an alert and returns it from getPendingAlerts", async () => {
    await enqueueAlert({
      id: "test-1",
      contractId: "CABC123",
      severity: "Critical",
      message: "Storage entry expiring in 2 ledgers",
      timestamp: new Date().toISOString(),
    });

    const pending = await getPendingAlerts();
    expect(pending).toHaveLength(1);
    expect(pending[0].id).toBe("test-1");
    expect(pending[0].dismissed).toBe(false);
    expect(pending[0].enqueuedAt).toBeTruthy();
  });

  it("no-ops on duplicate IDs (first write wins)", async () => {
    await enqueueAlert({
      id: "dup-id",
      contractId: "CXYZ",
      severity: "Info",
      message: "First",
      timestamp: new Date().toISOString(),
    });
    await enqueueAlert({
      id: "dup-id",
      contractId: "CXYZ",
      severity: "Warning",
      message: "Second — should be ignored",
      timestamp: new Date().toISOString(),
    });

    const pending = await getPendingAlerts();
    const ours = pending.filter((a) => a.id === "dup-id");
    expect(ours).toHaveLength(1);
    expect(ours[0].severity).toBe("Info"); // first write wins
  });
});

describe("alertQueue — dismissAlert / dismissAll", () => {
  it("marks a single alert as dismissed and removes it from getPendingAlerts", async () => {
    await enqueueAlert({
      id: "dismiss-me",
      contractId: "CABC",
      severity: "Warning",
      message: "Will be dismissed",
      timestamp: new Date().toISOString(),
    });
    await dismissAlert("dismiss-me");

    const pending = await getPendingAlerts();
    expect(pending.find((a) => a.id === "dismiss-me")).toBeUndefined();
  });

  it("dismissAll removes all pending alerts from the visible queue", async () => {
    for (let i = 0; i < 3; i++) {
      await enqueueAlert({
        id: `all-${i}`,
        contractId: "CABC",
        severity: "Critical",
        message: `Alert ${i}`,
        timestamp: new Date().toISOString(),
      });
    }

    await dismissAll();
    expect(await getPendingAlerts()).toHaveLength(0);
  });
});

describe("alertQueue — deleteAlert / purgeDismissed", () => {
  it("deleteAlert permanently removes from IDB", async () => {
    await enqueueAlert({
      id: "delete-me",
      contractId: "C1",
      severity: "Info",
      message: "Delete me",
      timestamp: new Date().toISOString(),
    });
    await deleteAlert("delete-me");

    const pending = await getPendingAlerts();
    expect(pending.find((a) => a.id === "delete-me")).toBeUndefined();
  });

  it("purgeDismissed cleans up only dismissed entries", async () => {
    await enqueueAlert({
      id: "keep",
      contractId: "C1",
      severity: "Info",
      message: "Keep me",
      timestamp: new Date().toISOString(),
    });
    await enqueueAlert({
      id: "purge",
      contractId: "C2",
      severity: "Warning",
      message: "Purge me after dismiss",
      timestamp: new Date().toISOString(),
    });
    await dismissAlert("purge");
    await purgeDismissed();

    const pending = await getPendingAlerts();
    expect(pending.find((a) => a.id === "keep")).toBeDefined();
    expect(pending.find((a) => a.id === "purge")).toBeUndefined();
  });
});

describe("alertQueue — getPendingCount", () => {
  it("returns the count of non-dismissed alerts", async () => {
    await enqueueAlert({
      id: "c1",
      contractId: "C",
      severity: "Info",
      message: "a",
      timestamp: new Date().toISOString(),
    });
    await enqueueAlert({
      id: "c2",
      contractId: "C",
      severity: "Critical",
      message: "b",
      timestamp: new Date().toISOString(),
    });
    await dismissAlert("c1");

    expect(await getPendingCount()).toBe(1);
  });
});
