/**
 * Tests for apps/web/components/LastUpdated.tsx, the footer timestamp that
 * most dashboard pages show, plus a check that the (app) layout renders it.
 */

import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { recordLastUpdated, resetLastUpdated } from "@/lib/lastUpdated";
import { LastUpdated } from "./LastUpdated";

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

vi.mock("@/components/NetworkSelector", () => ({
  NetworkSelector: () => <div data-testid="network-selector" />,
}));

vi.mock("@/components/ThemeToggle", () => ({
  ThemeToggle: () => <div data-testid="theme-toggle" />,
}));

describe("LastUpdated", () => {
  beforeEach(() => {
    resetLastUpdated();
  });

  afterEach(() => {
    cleanup();
    resetLastUpdated();
  });

  it("shows a placeholder before any data has been fetched", () => {
    render(<LastUpdated />);
    const el = screen.getByTestId("last-updated");
    expect(el.dataset.state).toBe("pending");
    expect(el.textContent).toContain("Last updated");
  });

  it("shows the fetch time, relative age, and resource once data arrives", () => {
    render(<LastUpdated />);

    const at = Date.now();
    act(() => {
      recordLastUpdated("watchdog", at);
    });

    const el = screen.getByTestId("last-updated");
    expect(el.dataset.state).toBe("ready");
    expect(el.textContent).toContain("watchdog");
    expect(el.textContent).toContain("just now");
    expect(el.querySelector("time")?.getAttribute("datetime")).toBe(
      new Date(at).toISOString()
    );
  });

  it("advances the timestamp after each later fetch", () => {
    render(<LastUpdated />);

    const first = Date.now() - 5 * 60_000;
    act(() => {
      recordLastUpdated("contracts", first);
    });
    expect(
      screen
        .getByTestId("last-updated")
        .querySelector("time")
        ?.getAttribute("datetime")
    ).toBe(new Date(first).toISOString());

    const second = first + 60_000;
    act(() => {
      recordLastUpdated("watchlist", second);
    });

    const el = screen.getByTestId("last-updated");
    expect(el.textContent).toContain("watchlist");
    expect(el.querySelector("time")?.getAttribute("datetime")).toBe(
      new Date(second).toISOString()
    );
  });
});

describe("AppLayout footer", () => {
  afterEach(() => {
    cleanup();
    resetLastUpdated();
  });

  it("renders the last updated footer on every dashboard page", async () => {
    const { default: AppLayout } = await import("@/app/(app)/layout");
    render(
      <AppLayout>
        <p>page content</p>
      </AppLayout>
    );

    const footer = screen.getByRole("contentinfo");
    expect(footer.textContent).toContain("Last updated");
  });
});
