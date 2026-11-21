/**
 * Tests for apps/web/components/SkipLink.tsx (issue #759).
 *
 * Covers the two acceptance criteria a unit test can hold: the link is the
 * first focusable control and activating it moves focus to #main.
 */

import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { SkipLink } from "./SkipLink";

/** Minimal page shape: skip link, site nav, then the main landmark. */
function Fixture() {
  return (
    <>
      <SkipLink />
      <nav aria-label="Primary">
        <a href="/contracts">Contracts</a>
      </nav>
      <main id="main" tabIndex={-1}>
        <h1>Dashboard</h1>
      </main>
    </>
  );
}

afterEach(() => {
  cleanup();
});

describe("SkipLink", () => {
  it("is the first focusable control and targets #main", () => {
    const { container } = render(<Fixture />);

    const link = screen.getByRole("link", { name: "Skip to main content" });
    expect(link).toHaveAttribute("href", "#main");

    const firstFocusable = container.querySelector<HTMLElement>(
      "a, button, input, select, textarea, [tabindex]"
    );
    expect(firstFocusable).toBe(link);
  });

  it("moves focus to the main landmark when activated", () => {
    render(<Fixture />);

    const main = screen.getByRole("main");
    expect(main).not.toHaveFocus();

    fireEvent.click(screen.getByRole("link", { name: "Skip to main content" }));

    expect(main).toHaveFocus();
  });

  it("stays a fragment link when the target is missing", () => {
    render(
      <>
        <SkipLink />
        <p>No main landmark here</p>
      </>
    );

    const link = screen.getByRole("link", { name: "Skip to main content" });
    expect(link).toHaveAttribute("href", "#main");
    // A missing target must not throw; the browser handles the fragment.
    expect(() => fireEvent.click(link)).not.toThrow();
  });

  it("accepts a custom target id", () => {
    render(
      <>
        <SkipLink targetId="content" />
        <main id="content" tabIndex={-1}>
          <h1>Content</h1>
        </main>
      </>
    );

    fireEvent.click(screen.getByRole("link", { name: "Skip to main content" }));
    expect(document.getElementById("content")).toHaveFocus();
  });
});
