import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { StrictMode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ThemeProvider, useTheme } from "./ThemeProvider";

// jsdom has no matchMedia; the provider only asks whether the OS prefers dark.
function stubMatchMedia(dark: boolean) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: dark && query.includes("dark"),
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
    onchange: null,
  }));
}

/**
 * Regression tests for the theme round-trip.
 *
 * The dev server runs React StrictMode, which mounts every effect twice. The
 * provider must not persist its default ("system") before it has read the
 * stored value, otherwise the second mount pass reads back the clobbered
 * default and a user's chosen theme is lost on reload.
 */

function Picker() {
  const { setTheme } = useTheme();
  return <button onClick={() => setTheme("dark")}>dark</button>;
}

function mount() {
  return render(
    <StrictMode>
      <ThemeProvider>
        <Picker />
      </ThemeProvider>
    </StrictMode>
  );
}

describe("ThemeProvider", () => {
  beforeEach(() => {
    stubMatchMedia(false);
    localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
  });

  afterEach(() => {
    cleanup();
    localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
  });

  it("applies a chosen theme to <html> and persists it", async () => {
    mount();

    fireEvent.click(screen.getByRole("button", { name: "dark" }));

    await waitFor(() =>
      expect(document.documentElement.getAttribute("data-theme")).toBe("dark")
    );
    expect(localStorage.getItem("theme")).toBe("dark");
  });

  it("keeps the stored theme across a remount (reload)", async () => {
    const first = mount();
    fireEvent.click(screen.getByRole("button", { name: "dark" }));
    await waitFor(() => expect(localStorage.getItem("theme")).toBe("dark"));
    first.unmount();

    mount();

    await waitFor(() =>
      expect(document.documentElement.getAttribute("data-theme")).toBe("dark")
    );
    expect(localStorage.getItem("theme")).toBe("dark");
  });

  it("applies a stored dark theme on the first render, with no system pass", async () => {
    localStorage.setItem("theme", "dark");
    // jsdom reports `prefers-color-scheme: dark` as false, so a stray
    // "system" apply is observable as a flip back to light.
    const observed: (string | null)[] = [];
    const observer = new MutationObserver(() =>
      observed.push(document.documentElement.getAttribute("data-theme"))
    );
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    mount();
    // MutationObserver callbacks are microtasks; let them drain.
    await new Promise((resolve) => setTimeout(resolve, 0));
    observer.disconnect();

    expect(observed).toEqual(["dark"]);
    expect(localStorage.getItem("theme")).toBe("dark");
  });
});
