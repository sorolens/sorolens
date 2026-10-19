/**
 * Tests for apps/web/app/page.tsx (the landing page).
 *
 * Guards the footer repository links (issue #653): the CLI and SDK live in
 * the monorepo, not in separate sorolens-cli / sorolens-sdk repositories.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

expect.extend(matchers);

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

vi.mock("@/components/LiveStats", () => ({
  LiveStats: () => <div data-testid="live-stats" />,
}));

describe("Landing page footer", () => {
  afterEach(() => {
    cleanup();
  });

  async function renderFooter() {
    const { default: Home } = await import("@/app/page");
    render(<Home />);
    return within(screen.getByRole("contentinfo"));
  }

  it("points CLI and SDK at their monorepo directories", async () => {
    const footer = await renderFooter();
    expect(footer.getByRole("link", { name: "CLI" })).toHaveAttribute(
      "href",
      "https://github.com/sorolens/sorolens/tree/main/cli"
    );
    expect(footer.getByRole("link", { name: "SDK" })).toHaveAttribute(
      "href",
      "https://github.com/sorolens/sorolens/tree/main/packages"
    );
  });

  it("links no GitHub repository outside the monorepo", async () => {
    const footer = await renderFooter();
    const githubLinks = footer
      .getAllByRole("link")
      .map((a) => a.getAttribute("href") ?? "")
      .filter((href) => href.startsWith("https://github.com/"));
    expect(githubLinks.length).toBeGreaterThan(0);
    for (const href of githubLinks) {
      expect(href).toMatch(/^https:\/\/github\.com\/sorolens\/sorolens(\/|$)/);
    }
  });
});
