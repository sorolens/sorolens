/**
 * Tests for apps/web/components/StatCard.tsx (issue #627).
 */

// Registers jest-dom matchers with vitest, including their types.
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { StatCard } from "./StatCard";

afterEach(() => {
  cleanup();
});

describe("StatCard", () => {
  it("renders the label, value and subtext", () => {
    render(<StatCard label="Events" value={1234} subtext="last 7 days" />);

    expect(screen.getByText("Events")).toBeVisible();
    expect(screen.getByText("1234")).toBeVisible();
    expect(screen.getByText("last 7 days")).toBeVisible();
  });

  it("omits the subtext when it is not provided", () => {
    render(<StatCard label="Events" value="5.2K" />);

    expect(screen.getByText("5.2K")).toBeVisible();
    expect(screen.queryByText("last 7 days")).not.toBeInTheDocument();
  });

  it("uses the neutral border for the default variant", () => {
    render(<StatCard label="Events" value={1} variant="default" />);

    const card = screen.getByText("Events").parentElement as HTMLElement;
    expect(card.className).toContain("border-[var(--color-border)]");
    expect(card.className).not.toContain("border-[var(--color-warning)]");
  });

  it.each([
    ["warning", "var(--color-warning)"],
    ["danger", "var(--color-danger)"],
    ["safe", "var(--color-safe)"],
  ] as const)(
    "applies the %s variant colour to the border and value",
    (variant, colour) => {
      render(<StatCard label="Alerts" value={3} variant={variant} />);

      const card = screen.getByText("Alerts").parentElement as HTMLElement;
      expect(card.className).toContain(`border-[${colour}]`);
      expect(screen.getByText("3").className).toContain(`text-[${colour}]`);
    }
  );

  it("lets tone override variant when both are set", () => {
    render(
      <StatCard label="Alerts" value={3} variant="warning" tone="danger" />
    );

    const card = screen.getByText("Alerts").parentElement as HTMLElement;
    expect(card.className).toContain("border-[var(--color-danger)]");
    expect(card.className).not.toContain("border-[var(--color-warning)]");
  });

  it("falls back to the default tone when neither prop is set", () => {
    render(<StatCard label="Alerts" value={3} />);

    const card = screen.getByText("Alerts").parentElement as HTMLElement;
    expect(card.className).toContain("border-[var(--color-border)]");
    expect(screen.getByText("3").className).not.toContain("text-[var(--color-");
  });
});
