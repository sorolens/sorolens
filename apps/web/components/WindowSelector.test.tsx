/**
 * Tests for apps/web/components/WindowSelector.tsx (issue #627).
 */

// Registers jest-dom matchers with vitest, including their types.
import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { WindowSelector } from "./WindowSelector";
import type { TimeWindow } from "@/lib/types";

afterEach(() => {
  cleanup();
});

describe("WindowSelector", () => {
  it("renders every window by default", () => {
    render(<WindowSelector selected="7d" onChange={() => {}} />);

    for (const label of ["24h", "7d", "30d", "All"]) {
      expect(screen.getByRole("button", { name: label })).toBeVisible();
    }
  });

  it("calls onChange with the clicked window's value", () => {
    const onChange = vi.fn<(window: TimeWindow) => void>();
    render(<WindowSelector selected="7d" onChange={onChange} />);

    fireEvent.click(screen.getByRole("button", { name: "30d" }));
    expect(onChange).toHaveBeenCalledWith("30d");

    // The "All" button reports the sentinel value, not its label.
    fireEvent.click(screen.getByRole("button", { name: "All" }));
    expect(onChange).toHaveBeenLastCalledWith("all");
  });

  it("highlights the selected window", () => {
    render(<WindowSelector selected="30d" onChange={() => {}} />);

    const selected =
      screen.getByRole("button", { name: "30d" }).className ?? "";
    const other = screen.getByRole("button", { name: "24h" }).className ?? "";
    expect(selected).toContain("bg-[var(--color-accent)]");
    expect(other).not.toContain("bg-[var(--color-accent)]");
  });

  it("renders only the requested subset, in the given order", () => {
    render(
      <WindowSelector
        selected="24h"
        onChange={() => {}}
        options={["all", "24h"]}
      />
    );

    expect(screen.getByRole("button", { name: "24h" })).toBeVisible();
    expect(screen.getByRole("button", { name: "All" })).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "7d" })
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "30d" })
    ).not.toBeInTheDocument();
  });

  it("drops unknown values from the options subset", () => {
    render(
      <WindowSelector
        selected="24h"
        onChange={() => {}}
        options={["24h", "nope" as TimeWindow]}
      />
    );

    expect(screen.getByRole("button", { name: "24h" })).toBeVisible();
    expect(screen.getAllByRole("button")).toHaveLength(1);
  });

  it("renders plain buttons so they are keyboard reachable", () => {
    render(<WindowSelector selected="7d" onChange={() => {}} />);

    for (const button of screen.getAllByRole("button")) {
      expect(button).toHaveAttribute("type", "button");
    }
  });
});
