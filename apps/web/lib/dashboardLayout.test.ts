import { describe, expect, it, beforeEach } from "vitest";
import {
  DASHBOARD_STORAGE_KEY,
  DEFAULT_LAYOUT,
  addWidget,
  availableWidgets,
  clearLayout,
  loadLayout,
  moveWidget,
  removeWidget,
  reorderWidget,
  resizeWidget,
  sanitizeLayout,
  saveLayout,
} from "./dashboardLayout";

/** Minimal in-memory Storage so the tests do not depend on jsdom quirks. */
function memoryStorage(): Storage {
  const map = new Map<string, string>();
  return {
    get length() {
      return map.size;
    },
    clear: () => map.clear(),
    getItem: (key) => (map.has(key) ? map.get(key)! : null),
    key: (index) => [...map.keys()][index] ?? null,
    removeItem: (key) => void map.delete(key),
    setItem: (key, value) => void map.set(key, String(value)),
  };
}

describe("sanitizeLayout", () => {
  it("keeps known widgets in order and normalises spans", () => {
    expect(
      sanitizeLayout([
        { id: "alerts", span: 1 },
        { id: "stats", span: 7 },
      ])
    ).toEqual([
      { id: "alerts", span: 1 },
      { id: "stats", span: 2 },
    ]);
  });

  it("drops unknown ids, duplicates and non-objects", () => {
    expect(
      sanitizeLayout([
        { id: "nope" },
        null,
        "stats",
        { id: "stats", span: 2 },
        { id: "stats", span: 1 },
      ])
    ).toEqual([{ id: "stats", span: 2 }]);
  });

  it("returns an empty layout for a non-array", () => {
    expect(sanitizeLayout({ id: "stats" })).toEqual([]);
    expect(sanitizeLayout(undefined)).toEqual([]);
  });
});

describe("persistence", () => {
  let storage: Storage;
  beforeEach(() => {
    storage = memoryStorage();
  });

  it("returns the default layout when nothing is stored", () => {
    expect(loadLayout(storage)).toEqual(DEFAULT_LAYOUT);
  });

  it("round-trips a custom layout and preserves an intentionally empty one", () => {
    const layout = [{ id: "alerts" as const, span: 2 as const }];
    saveLayout(layout, storage);
    expect(loadLayout(storage)).toEqual(layout);

    saveLayout([], storage);
    expect(loadLayout(storage)).toEqual([]);
  });

  it("falls back to defaults for corrupt stored data", () => {
    storage.setItem(DASHBOARD_STORAGE_KEY, "{not json");
    expect(loadLayout(storage)).toEqual(DEFAULT_LAYOUT);

    storage.setItem(DASHBOARD_STORAGE_KEY, JSON.stringify({ id: "stats" }));
    expect(loadLayout(storage)).toEqual(DEFAULT_LAYOUT);
  });

  it("clears the stored layout", () => {
    saveLayout([], storage);
    clearLayout(storage);
    expect(loadLayout(storage)).toEqual(DEFAULT_LAYOUT);
  });
});

describe("layout operations", () => {
  it("adds and removes widgets", () => {
    const withoutAlerts = removeWidget(DEFAULT_LAYOUT, "alerts");
    expect(withoutAlerts.map((w) => w.id)).not.toContain("alerts");
    expect(availableWidgets(withoutAlerts).map((w) => w.id)).toEqual([
      "alerts",
    ]);

    const restored = addWidget(withoutAlerts, "alerts");
    expect(restored.map((w) => w.id)).toContain("alerts");
    // Adding twice is a no-op.
    expect(addWidget(restored, "alerts")).toHaveLength(restored.length);
  });

  it("moves widgets with keyboard-style deltas and clamps at the ends", () => {
    const moved = moveWidget(DEFAULT_LAYOUT, "volume", -1);
    expect(moved.map((w) => w.id)).toEqual([
      "volume",
      "stats",
      "alerts",
      "hot_contracts",
    ]);
    expect(moveWidget(moved, "volume", -1)).toBe(moved);
  });

  it("reorders by drag index", () => {
    const moved = reorderWidget(DEFAULT_LAYOUT, 0, 2);
    expect(moved.map((w) => w.id)).toEqual([
      "volume",
      "alerts",
      "stats",
      "hot_contracts",
    ]);
  });

  it("resizes by toggling the span", () => {
    const resized = resizeWidget(DEFAULT_LAYOUT, "stats");
    expect(resized[0].span).toBe(1);
    expect(resizeWidget(resized, "stats")[0].span).toBe(2);
  });
});
