/**
 * Tests for apps/web/components/Spinner.tsx
 *
 * We use vitest + @testing-library/react + jsdom.
 */

import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, it, expect } from "vitest";
import { Spinner } from "./Spinner";

afterEach(() => {
  cleanup();
});

describe("Spinner", () => {
  it("renders an accessible loading indicator", () => {
    render(<Spinner />);
    expect(screen.getByRole("status", { name: "Loading" })).toBeInTheDocument();
  });

  it("defaults to the medium size", () => {
    render(<Spinner />);
    const spinner = screen.getByRole("status");
    expect(spinner).toHaveClass("h-6", "w-6");
  });

  it("applies the small size classes", () => {
    render(<Spinner size="sm" />);
    const spinner = screen.getByRole("status");
    expect(spinner).toHaveClass("h-4", "w-4");
  });

  it("applies the large size classes", () => {
    render(<Spinner size="lg" />);
    const spinner = screen.getByRole("status");
    expect(spinner).toHaveClass("h-8", "w-8");
  });

  it("renders with the spin animation and a transparent track", () => {
    render(<Spinner />);
    const spinner = screen.getByRole("status");
    expect(spinner).toHaveClass(
      "animate-spin",
      "rounded-full",
      "border-t-transparent"
    );
  });

  it("merges a custom className and forwards aria props", () => {
    render(
      <Spinner className="text-[var(--color-accent)]" aria-label="Fetching" />
    );
    const spinner = screen.getByRole("status", { name: "Fetching" });
    expect(spinner).toHaveClass("text-[var(--color-accent)]");
  });
});
