/**
 * Tests for apps/web/components/ThemeToggle.tsx (issue #627).
 */

// Registers jest-dom matchers with vitest, including their types.
import "@testing-library/jest-dom/vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ThemeToggle } from "./ThemeToggle";
import { ThemeProvider } from "./ThemeProvider";
import { renderToStaticMarkup } from "react-dom/server";

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

function renderToggle() {
  return render(
    <ThemeProvider>
      <ThemeToggle />
    </ThemeProvider>
  );
}

beforeEach(() => {
  stubMatchMedia(false);
  window.localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
});

afterEach(() => {
  cleanup();
  window.localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
  vi.restoreAllMocks();
});

describe("ThemeToggle", () => {
  it("reserves space with an empty placeholder during server render", () => {
    // The toggle renders a fixed-size placeholder on the server so the
    // header does not shift once the buttons mount after hydration.
    const markup = renderToStaticMarkup(
      <ThemeProvider>
        <ThemeToggle />
      </ThemeProvider>
    );

    expect(markup).toContain("h-8 w-24");
    expect(markup).not.toContain("Light");
    expect(markup).not.toContain("Dark");
    expect(markup).not.toContain("System");
  });

  it("renders the three labelled theme buttons after hydration", async () => {
    renderToggle();
    await act(async () => {}); // flush the mounted effect

    expect(screen.getByRole("button", { name: "Light mode" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Dark mode" })).toBeVisible();
    expect(screen.getByRole("button", { name: "System mode" })).toBeVisible();
  });

  it("switches the document theme and persists the choice", async () => {
    renderToggle();
    await act(async () => {});

    fireEvent.click(screen.getByRole("button", { name: "Dark mode" }));

    await waitFor(() =>
      expect(document.documentElement.getAttribute("data-theme")).toBe("dark")
    );
    expect(window.localStorage.getItem("theme")).toBe("dark");
  });

  it("applies a stored theme and switches back to light", async () => {
    window.localStorage.setItem("theme", "dark");
    renderToggle();
    await act(async () => {});

    await waitFor(() =>
      expect(document.documentElement.getAttribute("data-theme")).toBe("dark")
    );

    fireEvent.click(screen.getByRole("button", { name: "Light mode" }));
    await waitFor(() =>
      expect(document.documentElement.getAttribute("data-theme")).toBe("light")
    );
    expect(window.localStorage.getItem("theme")).toBe("light");
  });

  it("falls back to the system theme", async () => {
    stubMatchMedia(true); // OS prefers dark
    renderToggle();
    await act(async () => {});

    fireEvent.click(screen.getByRole("button", { name: "System mode" }));
    await waitFor(() =>
      expect(document.documentElement.getAttribute("data-theme")).toBe("dark")
    );
    expect(window.localStorage.getItem("theme")).toBe("system");
  });
});
