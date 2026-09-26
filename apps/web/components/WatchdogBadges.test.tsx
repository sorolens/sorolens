/**
 * Unit tests for WatchdogBadges components, including the new UptimeBadge
 * added by issue #187.
 */
import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { HealthBadge, SeverityBadge, UptimeBadge } from "./WatchdogBadges";

expect.extend(matchers);

afterEach(cleanup);

// ── HealthBadge ──────────────────────────────────────────────────────────────

describe("HealthBadge", () => {
  it("renders the status text", () => {
    render(<HealthBadge status="Healthy" />);
    expect(screen.getByText("Healthy")).toBeDefined();
  });

  it("renders an unknown status without crashing", () => {
    render(<HealthBadge status="CustomStatus" />);
    expect(screen.getByText("CustomStatus")).toBeDefined();
  });
});

// ── SeverityBadge ────────────────────────────────────────────────────────────

describe("SeverityBadge", () => {
  it("renders Critical severity", () => {
    render(<SeverityBadge severity="Critical" />);
    expect(screen.getByText("Critical")).toBeDefined();
  });

  it("renders Warning severity", () => {
    render(<SeverityBadge severity="Warning" />);
    expect(screen.getByText("Warning")).toBeDefined();
  });

  it("renders Info severity", () => {
    render(<SeverityBadge severity="Info" />);
    expect(screen.getByText("Info")).toBeDefined();
  });
});

// ── UptimeBadge ──────────────────────────────────────────────────────────────

describe("UptimeBadge", () => {
  it("shows a skeleton placeholder while loading (pct = null)", () => {
    render(<UptimeBadge window="24h" pct={null} />);
    // Should show the window label but not a percentage
    expect(screen.getByText(/24h/)).toBeDefined();
    // No numeric percentage should appear
    const text = screen.queryByText(/%/);
    expect(text).toBeNull();
  });

  it("renders uptime percentage with two decimal places", () => {
    render(<UptimeBadge window="24h" pct={99.98} />);
    expect(screen.getByText("99.98%")).toBeDefined();
  });

  it("renders 100.00% when fully up", () => {
    render(<UptimeBadge window="7d" pct={100} />);
    expect(screen.getByText("100.00%")).toBeDefined();
  });

  it("renders 0.00% when all checks failed", () => {
    render(<UptimeBadge window="30d" pct={0} />);
    expect(screen.getByText("0.00%")).toBeDefined();
  });

  it("shows the correct window label alongside the percentage", () => {
    const { container } = render(<UptimeBadge window="7d" pct={97.5} />);
    expect(container.textContent).toContain("7d");
    expect(container.textContent).toContain("97.50%");
  });

  it("has a title attribute describing the window", () => {
    const { container } = render(<UptimeBadge window="30d" pct={99} />);
    const badge = container.firstChild as HTMLElement;
    expect(badge.getAttribute("title")).toContain("30d");
  });

  it("renders all three windows correctly", () => {
    const windows = [
      { window: "24h" as const, pct: 99.98 },
      { window: "7d" as const, pct: 99.85 },
      { window: "30d" as const, pct: 98.5 },
    ];
    const { container } = render(
      <div>
        {windows.map(({ window, pct }) => (
          <UptimeBadge key={window} window={window} pct={pct} />
        ))}
      </div>
    );
    expect(container.textContent).toContain("99.98%");
    expect(container.textContent).toContain("99.85%");
    expect(container.textContent).toContain("98.50%");
  });
});
