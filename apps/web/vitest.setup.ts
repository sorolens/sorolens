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
