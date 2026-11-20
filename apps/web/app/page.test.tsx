/**
 * Tests for apps/web/app/page.tsx (the landing page).
 *
 * Guards the footer repository links (issue #653): the CLI and SDK live in
 * the monorepo, not in separate sorolens-cli / sorolens-sdk repositories.
 */

import * as matchers from "@testing-library/jest-dom/matchers";
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { EXTENSION_INSTALL_URL, EXTENSION_RELEASES_URL } from "@/lib/extension";

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

/**
 * The extension is an acquisition channel a visitor cannot find unless the
 * landing page says it exists (issue #764), so it is linked above the fold and
 * explained in one sentence.
 */
describe("Landing page landmarks", () => {
  afterEach(() => {
    cleanup();
  });

  it("renders exactly one main landmark targeted by the skip link", async () => {
    const { default: Home } = await import("@/app/page");
    render(<Home />);

    const mains = screen.getAllByRole("main");
    expect(mains).toHaveLength(1);
    expect(mains[0]).toHaveAttribute("id", "main");
  });

  it("labels the landing navigation", async () => {
    const { default: Home } = await import("@/app/page");
    render(<Home />);

    expect(screen.getByRole("navigation", { name: "Primary" })).toBeVisible();
  });
});

describe("Landing page extension callout", () => {
  afterEach(() => {
    cleanup();
  });

  it("offers the extension with a description of what it does", async () => {
    const { default: Home } = await import("@/app/page");
    render(<Home />);

    const callout = screen.getByTestId("extension-callout");
    expect(
      within(callout).getByRole("link", { name: "Get the extension" })
    ).toHaveAttribute("href", EXTENSION_INSTALL_URL);
    expect(
      within(callout).getByRole("link", { name: "Download a release build" })
    ).toHaveAttribute("href", EXTENSION_RELEASES_URL);
    expect(callout.textContent).toContain("Track on Sorolens");
    expect(callout.textContent).toContain("Stellar Expert");
  });
});
