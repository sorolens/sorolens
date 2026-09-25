/**
 * Offline alert queue backed by IndexedDB (issue #275).
 *
 * Alerts received while the user is offline are persisted here and replayed /
 * flushed once connectivity is restored.  The queue is intentionally
 * framework-agnostic so it can be used from both React components and the
 * service worker's push-event handler.
 */

import { openDB, type DBSchema, type IDBPDatabase } from "idb";

const DB_NAME = "sorolens-pwa";
const DB_VERSION = 1;
const STORE = "offline-alerts";

/** Shape stored in IndexedDB. */
export interface QueuedAlert {
  /** Unique ID derived from the alert payload or a crypto random fallback. */
  id: string;
  contractId: string;
  severity: "Info" | "Warning" | "Critical";
  message: string;
  timestamp: string;
  /** ISO-8601 time the alert was enqueued locally. */
  enqueuedAt: string;
  /** Whether the user has dismissed this alert from the offline queue UI. */
  dismissed: boolean;
}

interface SorolensDB extends DBSchema {
  [STORE]: {
    key: string;
    value: QueuedAlert;
    indexes: {
      by_dismissed: number;
      by_enqueuedAt: string;
    };
  };
}

let _db: IDBPDatabase<SorolensDB> | null = null;

async function getDB(): Promise<IDBPDatabase<SorolensDB>> {
  if (_db) return _db;
  _db = await openDB<SorolensDB>(DB_NAME, DB_VERSION, {
    upgrade(db) {
      if (!db.objectStoreNames.contains(STORE)) {
        const store = db.createObjectStore(STORE, { keyPath: "id" });
        store.createIndex("by_dismissed", "dismissed");
        store.createIndex("by_enqueuedAt", "enqueuedAt");
      }
    },
  });
  return _db;
}

/**
 * Close the open IDB connection and reset the singleton.
 * Used only by test setup to achieve inter-test isolation without
 * deleting the database (which can block if a connection is still open).
 * @internal Do not import from production code.
 */
export async function _closeForTests(): Promise<void> {
  if (_db) {
    _db.close();
    _db = null;
  }
}

/** Enqueue an alert into the offline queue. No-ops on duplicate IDs. */
export async function enqueueAlert(
  alert: Omit<QueuedAlert, "enqueuedAt" | "dismissed">
): Promise<void> {
  const db = await getDB();
  const existing = await db.get(STORE, alert.id);
  if (existing) return; // already queued
  await db.put(STORE, {
    ...alert,
    enqueuedAt: new Date().toISOString(),
    dismissed: false,
  });
}

/** Return all non-dismissed alerts ordered newest-first. */
export async function getPendingAlerts(): Promise<QueuedAlert[]> {
  const db = await getDB();
  const all = await db.getAll(STORE);
  return all
    .filter((a) => !a.dismissed)
    .sort((a, b) => b.enqueuedAt.localeCompare(a.enqueuedAt));
}

/** Return the total count of non-dismissed queued alerts. */
export async function getPendingCount(): Promise<number> {
  const pending = await getPendingAlerts();
  return pending.length;
}

/** Mark an alert as dismissed (keeps it in IDB for history). */
export async function dismissAlert(id: string): Promise<void> {
  const db = await getDB();
  const alert = await db.get(STORE, id);
  if (!alert) return;
  await db.put(STORE, { ...alert, dismissed: true });
}

/** Dismiss all pending alerts at once. */
export async function dismissAll(): Promise<void> {
  const pending = await getPendingAlerts();
  await Promise.all(pending.map((a) => dismissAlert(a.id)));
}

/** Permanently delete a single alert from IDB. */
export async function deleteAlert(id: string): Promise<void> {
  const db = await getDB();
  await db.delete(STORE, id);
}

/** Permanently delete all dismissed alerts (housekeeping). */
export async function purgeDismissed(): Promise<void> {
  const db = await getDB();
  const all = await db.getAll(STORE);
  await Promise.all(
    all.filter((a) => a.dismissed).map((a) => db.delete(STORE, a.id))
  );
}
