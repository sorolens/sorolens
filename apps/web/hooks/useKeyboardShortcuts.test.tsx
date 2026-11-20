import { act, cleanup, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useKeyboardShortcuts } from "./useKeyboardShortcuts";
import {
  OPEN_SEARCH_EVENT,
  SHORTCUTS_CHANGED_EVENT,
  SHORTCUTS_STORAGE_KEY,
  type ShortcutBinding,
} from "@/lib/keyboard-shortcuts";

function press(key: string, init: KeyboardEventInit = {}): KeyboardEvent {
  const event = new KeyboardEvent("keydown", {
    key,
    bubbles: true,
    cancelable: true,
    ...init,
  });
  document.body.dispatchEvent(event);
  return event;
}

function pressOn(target: Element, key: string, init: KeyboardEventInit = {}) {
  const event = new KeyboardEvent("keydown", {
    key,
    bubbles: true,
    cancelable: true,
    ...init,
  });
  target.dispatchEvent(event);
  return event;
}

const bindings = (
  run: () => void,
  searchRun: () => void = vi.fn()
): ShortcutBinding[] => [
  { id: "contracts", sequence: "g c", label: "Contracts", run },
  { id: "search", sequence: "/", label: "Search", run: searchRun },
];

describe("useKeyboardShortcuts", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("runs a sequence binding", () => {
    const run = vi.fn();
    renderHook(() => useKeyboardShortcuts(bindings(run)));

    press("g");
    expect(run).not.toHaveBeenCalled();
    press("c");
    expect(run).toHaveBeenCalledTimes(1);
  });

  it("runs a single-key binding and prevents the default", () => {
    const run = vi.fn();
    const searchRun = vi.fn();
    renderHook(() => useKeyboardShortcuts(bindings(run, searchRun)));

    const event = press("/");
    expect(searchRun).toHaveBeenCalledTimes(1);
    expect(event.defaultPrevented).toBe(true);
  });

  it("ignores Cmd, Ctrl and Alt combinations", () => {
    const run = vi.fn();
    renderHook(() => useKeyboardShortcuts(bindings(run)));

    press("g", { metaKey: true });
    press("c", { metaKey: true });
    press("g", { ctrlKey: true });
    press("c", { ctrlKey: true });
    press("g", { altKey: true });
    press("c", { altKey: true });

    expect(run).not.toHaveBeenCalled();
  });

  it("ignores keys typed into form fields", () => {
    const run = vi.fn();
    renderHook(() => useKeyboardShortcuts(bindings(run)));

    const input = document.createElement("input");
    document.body.appendChild(input);
    try {
      pressOn(input, "g");
      pressOn(input, "c");
      expect(run).not.toHaveBeenCalled();
    } finally {
      input.remove();
    }
  });

  it("ignores keys during IME composition", () => {
    const run = vi.fn();
    renderHook(() => useKeyboardShortcuts(bindings(run)));

    press("g", { isComposing: true });
    press("c", { isComposing: true });

    expect(run).not.toHaveBeenCalled();
  });

  it("does nothing when shortcuts were disabled in Settings", () => {
    localStorage.setItem(SHORTCUTS_STORAGE_KEY, "1");
    const run = vi.fn();
    renderHook(() => useKeyboardShortcuts(bindings(run)));

    press("g");
    press("c");
    expect(run).not.toHaveBeenCalled();
  });

  it("re-enables when the preference changes without a reload", () => {
    localStorage.setItem(SHORTCUTS_STORAGE_KEY, "1");
    const run = vi.fn();
    renderHook(() => useKeyboardShortcuts(bindings(run)));

    press("g");
    press("c");
    expect(run).not.toHaveBeenCalled();

    // Settings writes the flag then announces it; the hook re-reads and
    // re-attaches its listener.
    act(() => {
      localStorage.setItem(SHORTCUTS_STORAGE_KEY, "0");
      window.dispatchEvent(new Event(SHORTCUTS_CHANGED_EVENT));
    });

    press("g");
    press("c");
    expect(run).toHaveBeenCalledTimes(1);
  });

  it("stays inert when the caller disables it", () => {
    const run = vi.fn();
    renderHook(() => useKeyboardShortcuts(bindings(run), { enabled: false }));

    press("g");
    press("c");
    expect(run).not.toHaveBeenCalled();
  });
});

describe("search event", () => {
  it("uses a namespaced event name", () => {
    // Guards against accidentally renaming the event the palette listens for.
    expect(OPEN_SEARCH_EVENT).toBe("sorolens:open-search");
  });
});
