/**
 * Integration tests for the shell-level shortcut wiring (issue #831).
 *
 * Unlike the hook unit tests, these render the real `AppShortcuts` component
 * and assert it drives the Next router and the command palette, so the layout,
 * binding table and dialog are covered together.
 */

import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AppShortcuts } from "./AppShortcuts";
import {
  OPEN_SEARCH_EVENT,
  SHORTCUTS_STORAGE_KEY,
} from "@/lib/keyboard-shortcuts";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

function press(key: string, init: KeyboardEventInit = {}) {
  // Wrapped in act so state updates made by shortcut handlers (such as
  // opening the help dialog) flush before assertions.
  act(() => {
    document.body.dispatchEvent(
      new KeyboardEvent("keydown", {
        key,
        bubbles: true,
        cancelable: true,
        ...init,
      })
    );
  });
}

function sequence(...keys: string[]) {
  for (const key of keys) press(key);
}

describe("AppShortcuts", () => {
  beforeEach(() => {
    localStorage.clear();
    push.mockClear();
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it("g then c routes to Contracts", () => {
    render(<AppShortcuts />);
    sequence("g", "c");
    expect(push).toHaveBeenCalledWith("/contracts");
  });

  it("g then e routes to Events", () => {
    render(<AppShortcuts />);
    sequence("g", "e");
    expect(push).toHaveBeenCalledWith("/events");
  });

  it("g then i routes to Invocations", () => {
    render(<AppShortcuts />);
    sequence("g", "i");
    expect(push).toHaveBeenCalledWith("/invocations");
  });

  it("/ asks the command palette to open", () => {
    const listener = vi.fn();
    window.addEventListener(OPEN_SEARCH_EVENT, listener);
    try {
      render(<AppShortcuts />);
      press("/");
      expect(listener).toHaveBeenCalledTimes(1);
    } finally {
      window.removeEventListener(OPEN_SEARCH_EVENT, listener);
    }
  });

  it("? opens the help dialog and Escape closes it", () => {
    render(<AppShortcuts />);

    press("?");
    const dialog = screen.getByRole("dialog", { name: "Keyboard shortcuts" });
    expect(dialog).toBeInTheDocument();
    expect(dialog).toHaveTextContent("Go to Contracts");

    press("Escape");
    expect(
      screen.queryByRole("dialog", { name: "Keyboard shortcuts" })
    ).not.toBeInTheDocument();
  });

  it("does not navigate when shortcuts are disabled", () => {
    localStorage.setItem(SHORTCUTS_STORAGE_KEY, "1");
    render(<AppShortcuts />);

    sequence("g", "c");
    expect(push).not.toHaveBeenCalled();
  });

  it("does not navigate Cmd/Ctrl combinations", () => {
    render(<AppShortcuts />);

    press("g", { metaKey: true });
    press("c", { metaKey: true });

    expect(push).not.toHaveBeenCalled();
  });
});
