/**
 * Tests for apps/web/components/settings/ThemeSection.tsx (issue #765).
 *
 * Theme is owned by `ThemeProvider`, so the card must drive that provider
 * rather than keep a second copy. Asserting on `data-theme` and the stored
 * `theme` key checks the wiring end to end.
 */

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
import { ThemeSection } from "./ThemeSection";
import { ThemeProvider } from "@/components/ThemeProvider";

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

function renderSection(status: "idle" | "saved" | "error" = "idle") {
  const onSave = vi.fn(() => true);
  render(
    <ThemeProvider>
      <ThemeSection status={status} onSave={onSave} />
    </ThemeProvider>
  );
  return { onSave };
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

describe("ThemeSection", () => {
  it("renders the three theme radios in a labelled card", async () => {
    renderSection();
    await act(async () => {}); // flush the provider's mount effect

    expect(screen.getByRole("heading", { name: "Theme" })).toBeVisible();
    expect(screen.getByRole("radio", { name: "Light" })).toBeVisible();
    expect(screen.getByRole("radio", { name: "Dark" })).toBeVisible();
    expect(screen.getByRole("radio", { name: "System" })).toBeVisible();
  });

  it("checks the radio that matches the current theme", async () => {
    window.localStorage.setItem("theme", "dark");
    renderSection();
    await act(async () => {});

    expect(screen.getByRole("radio", { name: "Dark" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "Light" })).not.toBeChecked();
  });

  it("applies and persists a theme through the provider", async () => {
    renderSection();
    await act(async () => {});

    fireEvent.click(screen.getByRole("radio", { name: "Dark" }));

    await waitFor(() =>
      expect(document.documentElement.getAttribute("data-theme")).toBe("dark")
    );
    expect(window.localStorage.getItem("theme")).toBe("dark");
    expect(screen.getByRole("radio", { name: "Dark" })).toBeChecked();
  });

  it("keeps a previous theme when a failed save is reported", async () => {
    window.localStorage.setItem("theme", "light");
    renderSection("error");
    await act(async () => {});

    expect(screen.getByRole("status")).toHaveTextContent(
      "Could not save — storage unavailable"
    );
    expect(screen.getByRole("radio", { name: "Light" })).toBeChecked();
  });

  it("saves through the card's Save button", async () => {
    const { onSave } = renderSection();
    await act(async () => {});

    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(onSave).toHaveBeenCalledTimes(1);
  });
});
