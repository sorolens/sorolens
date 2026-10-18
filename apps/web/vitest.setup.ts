/**
 * vitest.setup.ts — global test setup for apps/web.
 *
 * Installs fake-indexeddb as the global IndexedDB implementation so tests
 * that import alertQueue.ts work in jsdom without a real browser.
 *
 * Each test file runs in its own worker (default vitest behaviour) which gets
 * its own globalThis — so the "sorolens-pwa" IDB database is completely
 * isolated between test files.  Within a single file, vi.resetModules() in
 * the file's own beforeEach resets the `_db` singleton so each test opens a
 * fresh connection to a fresh database name (see alertQueue.test.ts).
 */
import "fake-indexeddb/auto";
import * as matchers from "@testing-library/jest-dom/matchers";
import { expect } from "vitest";

expect.extend(matchers);

class MockStorage implements Storage {
  private store = new Map<string, string>();
  get length() {
    return this.store.size;
  }
  clear() {
    this.store.clear();
  }
  getItem(key: string) {
    return this.store.has(key) ? this.store.get(key)! : null;
  }
  setItem(key: string, value: string) {
    this.store.set(key, String(value));
  }
  removeItem(key: string) {
    this.store.delete(key);
  }
  key(index: number) {
    return Array.from(this.store.keys())[index] ?? null;
  }
}

const storageInstance = new MockStorage();

if (typeof window !== "undefined") {
  Object.defineProperty(window, "localStorage", {
    value: storageInstance,
    writable: true,
    configurable: true,
  });
}

try {
  delete (globalThis as any).localStorage;
} catch {}

Object.defineProperty(globalThis, "localStorage", {
  value: storageInstance,
  writable: true,
  configurable: true,
});
