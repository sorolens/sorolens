/**
 * Tests for apps/web/components/Breadcrumbs.tsx
 *
 * We use vitest + @testing-library/react + jsdom.
 * next/link is mocked to a plain <a> so we don't need the Next.js runtime.
 * next/navigation is mocked so each test controls the current route and params.
 */

import type { AnchorHTMLAttributes, ReactNode } from "react";
// Registers jest-dom matchers with vitest, including their types.
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Breadcrumbs } from "./Breadcrumbs";

// ── Mock next/link ──────────────────────────────────────────────────────────
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...rest
  }: { children: ReactNode; href: string } & AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

// ── Mock next/navigation ────────────────────────────────────────────────────
// The factory dereferences `route` lazily, so tests can point it at any route.
const route = {
  pathname: "/",
  params: {} as Record<string, string | string[]>,
};

vi.mock("next/navigation", () => ({
  usePathname: () => route.pathname,
  useParams: () => route.params,
}));

// ── Fixtures ─────────────────────────────────────────────────────────────────

const LONG_CONTRACT_ID =
  "CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C33";

// Head 6 + "..." + tail 4, matching the MonoId truncation style.
const TRUNCATED_CONTRACT_ID = "CAVRQG...5C33";

// ── Tests ────────────────────────────────────────────────────────────────────

describe("Breadcrumbs", () => {
  beforeEach(() => {
    route.pathname = "/";
    route.params = {};
  });

  afterEach(() => {
    cleanup();
  });

  it("renders the full trail as links on a nested contracts route", () => {
    route.pathname = `/contracts/${LONG_CONTRACT_ID}`;
    route.params = { id: LONG_CONTRACT_ID };
    render(<Breadcrumbs />);

    const nav = screen.getByRole("navigation", { name: "Breadcrumb" });
    const links = within(nav).getAllByRole("link");
    expect(links).toHaveLength(3);

    expect(links[0]).toHaveTextContent("Home");
    expect(links[0]).toHaveAttribute("href", "/");

    expect(links[1]).toHaveTextContent("Contracts");
    expect(links[1]).toHaveAttribute("href", "/contracts");

    // Long ids are truncated in the label but keep the full value in the
    // href and the title.
    expect(links[2]).toHaveTextContent(TRUNCATED_CONTRACT_ID);
    expect(links[2]).toHaveAttribute("href", `/contracts/${LONG_CONTRACT_ID}`);
    expect(links[2]).toHaveAttribute("title", LONG_CONTRACT_ID);
  });

  it("marks the current page crumb with aria-current and keeps it a link", () => {
    route.pathname = "/watchdog/42";
    route.params = { id: "42" };
    render(<Breadcrumbs />);

    const nav = screen.getByRole("navigation", { name: "Breadcrumb" });
    const links = within(nav).getAllByRole("link");
    expect(links).toHaveLength(3);
    for (const link of links) {
      expect(link).toHaveAttribute("href");
    }

    const current = within(nav).getByRole("link", { name: "42" });
    expect(current).toHaveAttribute("href", "/watchdog/42");
    expect(current).toHaveAttribute("aria-current", "page");
    // Short ids stay readable instead of being truncated.
    expect(current).toHaveTextContent("42");
  });

  it("renders Home and the section crumb on a section route", () => {
    route.pathname = "/watchlist";
    route.params = {};
    render(<Breadcrumbs />);

    const nav = screen.getByRole("navigation", { name: "Breadcrumb" });
    const links = within(nav).getAllByRole("link");
    expect(links).toHaveLength(2);

    expect(links[0]).toHaveTextContent("Home");
    expect(links[0]).toHaveAttribute("href", "/");

    expect(links[1]).toHaveTextContent("Watchlist");
    expect(links[1]).toHaveAttribute("href", "/watchlist");
    expect(links[1]).toHaveAttribute("aria-current", "page");
  });

  it("renders nothing on the dashboard root", () => {
    route.pathname = "/";
    const { container } = render(<Breadcrumbs />);
    expect(container).toBeEmptyDOMElement();
  });
});
