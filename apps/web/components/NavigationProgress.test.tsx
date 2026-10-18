/**
 * Tests for apps/web/components/NavigationProgress.tsx
 *
 * We use vitest + @testing-library/react + jsdom.
 * next/navigation is mocked so each test controls the current pathname and
 * searchParams independently, following the pattern in Breadcrumbs.test.tsx.
 */

import "@testing-library/jest-dom/vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NavigationProgress } from "./NavigationProgress";

// ── Mock next/navigation ─────────────────────────────────────────────────────
const route = {
  pathname: "/",
  searchParams: new URLSearchParams(),
};

vi.mock("next/navigation", () => ({
  usePathname: () => route.pathname,
  useSearchParams: () => route.searchParams,
}));

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Re-renders the component after updating the mocked route. */
function navigate(pathname: string, params: Record<string, string> = {}): void {
  route.pathname = pathname;
  route.searchParams = new URLSearchParams(params);
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe("NavigationProgress", () => {
  beforeEach(() => {
    route.pathname = "/";
    route.searchParams = new URLSearchParams();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    cleanup();
  });

  it("renders nothing on initial mount (no navigation yet)", () => {
    const { container } = render(<NavigationProgress />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows a progressbar after a route change", () => {
    const { rerender } = render(<NavigationProgress />);

    act(() => {
      navigate("/contracts");
    });
    rerender(<NavigationProgress />);

    const bar = screen.getByRole("progressbar", { name: "Page loading" });
    expect(bar).toBeInTheDocument();
  });

  it("hides the bar after the completion timer fires", () => {
    const { rerender } = render(<NavigationProgress />);

    act(() => {
      navigate("/contracts");
    });
    rerender(<NavigationProgress />);

    expect(screen.getByRole("progressbar")).toBeInTheDocument();

    // Advance past TRANSITION_MS (200) + COMPLETE_HOLD_MS (300).
    act(() => {
      vi.advanceTimersByTime(600);
    });
    rerender(<NavigationProgress />);

    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  });

  it("sets aria-valuenow=100 in the completing phase", () => {
    const { rerender } = render(<NavigationProgress />);

    act(() => {
      navigate("/watchdog");
    });
    rerender(<NavigationProgress />);

    const bar = screen.getByRole("progressbar");
    // On route-change render the bar enters 'completing' immediately.
    expect(bar).toHaveAttribute("aria-valuenow", "100");
    expect(bar).toHaveAttribute("aria-valuemin", "0");
    expect(bar).toHaveAttribute("aria-valuemax", "100");
  });

  it("resets when a second navigation fires before the first completes", () => {
    const { rerender } = render(<NavigationProgress />);

    act(() => {
      navigate("/contracts");
    });
    rerender(<NavigationProgress />);

    // Advance partway through the hold.
    act(() => {
      vi.advanceTimersByTime(200);
    });

    // Second navigation fires before the bar fully disappears.
    act(() => {
      navigate("/watchlist");
    });
    rerender(<NavigationProgress />);

    // Bar should still be visible (reset rather than hidden).
    expect(screen.getByRole("progressbar")).toBeInTheDocument();

    // Advance past the full cycle of the second navigation.
    act(() => {
      vi.advanceTimersByTime(600);
    });
    rerender(<NavigationProgress />);

    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  });

  it("does not show the bar if pathname hasn't changed", () => {
    const { rerender } = render(<NavigationProgress />);

    // Re-render without changing the route — simulates a state update
    // in a parent component.
    rerender(<NavigationProgress />);

    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  });
});
