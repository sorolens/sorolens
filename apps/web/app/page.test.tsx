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

/**
 * Invocations, Compare, Watchlist and Reports were reachable only by typing
 * the URL (issue #647), so the landing page links each of them.
 */
describe("Landing page dashboard links", () => {
  afterEach(() => {
    cleanup();
  });

  it("links every dashboard page that has no feature card", async () => {
    const { default: Home } = await import("@/app/page");
    render(<Home />);

    const explore = within(
      screen.getByRole("navigation", { name: "Explore the dashboard" })
    );
    expect(explore.getByRole("link", { name: "Invocations" })).toHaveAttribute(
      "href",
      "/invocations"
    );
    expect(
      explore.getByRole("link", { name: "Compare contracts" })
    ).toHaveAttribute("href", "/compare");
    expect(explore.getByRole("link", { name: "Watchlist" })).toHaveAttribute(
      "href",
      "/watchlist"
    );
    expect(explore.getByRole("link", { name: "Reports" })).toHaveAttribute(
      "href",
      "/watchdog/reports"
    );
    expect(
      explore.getByRole("link", { name: "API playground" })
    ).toHaveAttribute("href", "/playground");
    expect(
      explore.getByRole("link", { name: "Documentation" })
    ).toHaveAttribute("href", "/docs");
  });
});

/**
 * A visitor should be able to try the product without reading the README:
 * the landing page shows real screenshots and copyable CLI/API snippets.
 */
describe("Landing page quickstart and screenshots", () => {
  afterEach(() => {
    cleanup();
  });

  it("offers copyable CLI and API snippets with next steps", async () => {
    const { default: Home } = await import("@/app/page");
    render(<Home />);

    expect(
      screen.getByRole("button", { name: "Copy CLI quickstart" })
    ).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Copy API request" })
    ).toBeVisible();
    expect(screen.getByText(/sorolens track/)).toBeVisible();
    expect(screen.getByText(/api\/v1\/contracts/)).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Getting started guide" })
    ).toHaveAttribute("href", "/docs/getting-started");
    expect(
      screen.getByRole("link", { name: "Open the API playground" })
    ).toHaveAttribute("href", "/playground");
  });

  it("shows real product screenshots with descriptive alt text", async () => {
    const { default: Home } = await import("@/app/page");
    render(<Home />);

    const shots = screen.getAllByTestId("landing-screenshot");
    expect(shots).toHaveLength(3);
    for (const shot of shots) {
      const image = within(shot).getByRole("img");
      expect(image).toHaveAttribute("alt");
      expect(image.getAttribute("alt")).not.toBe("");
    }
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
