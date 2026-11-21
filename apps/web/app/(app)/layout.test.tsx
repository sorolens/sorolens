/**
 * Tests for apps/web/app/(app)/layout.tsx
 *
 * The layout wraps every dashboard page, so asserting on it covers them all.
 */

import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// The layout marks the active link from the current path; point it at a route
// per test. The factory reads `route` lazily so hoisting is safe.
const route = { pathname: "/dashboard" };

vi.mock("next/navigation", () => ({
  usePathname: () => route.pathname,
  useParams: () => ({}),
  // CmdkSearch, rendered by the header, resolves a router on mount.
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    prefetch: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    refresh: vi.fn(),
  }),
}));

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...rest
  }: {
    children: React.ReactNode;
    href: string;
  } & React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

vi.mock("@/components/NetworkSelector", () => ({
  NetworkSelector: () => <div data-testid="network-selector" />,
}));

vi.mock("@/components/ThemeToggle", () => ({
  ThemeToggle: () => <div data-testid="theme-toggle" />,
}));

// The header search is unrelated to the shell's landmarks; it also needs a
// ThemeProvider, which this unit test deliberately does not mount.
vi.mock("@/components/CmdkSearch", () => ({
  CmdkSearch: () => <div data-testid="cmdk-search" />,
}));

describe("AppLayout", () => {
  beforeEach(() => {
    route.pathname = "/dashboard";
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  async function renderLayout() {
    const { default: AppLayout } = await import("@/app/(app)/layout");
    return render(
      <AppLayout>
        <p>page content</p>
      </AppLayout>
    );
  }

  it("renders the Stellar community line in the footer", async () => {
    await renderLayout();
    const footer = screen.getByRole("contentinfo");
    expect(footer.textContent).toContain(
      "Built for the Stellar developer community."
    );
  });

  it("links to reports and public system status", async () => {
    await renderLayout();
    expect(
      screen.getByRole("link", { name: "Reports" }).getAttribute("href")
    ).toBe("/watchdog/reports");
    expect(
      screen.getByRole("link", { name: "System status" }).getAttribute("href")
    ).toBe("/status");
  });

  it("renders one labelled primary nav and main landmark", async () => {
    await renderLayout();

    expect(screen.getByRole("navigation", { name: "Primary" })).toBeVisible();
    const mains = screen.getAllByRole("main");
    expect(mains).toHaveLength(1);
    expect(mains[0]).toHaveAttribute("id", "main");
  });

  it("marks the active navigation link with aria-current", async () => {
    route.pathname = "/contracts";
    await renderLayout();

    const nav = screen.getByRole("navigation", { name: "Primary" });
    expect(
      within(nav).getByRole("link", { name: "Contracts" })
    ).toHaveAttribute("aria-current", "page");
    expect(
      within(nav).getByRole("link", { name: "Events" })
    ).not.toHaveAttribute("aria-current");
    expect(
      within(nav).getByRole("link", { name: "Dashboard" })
    ).not.toHaveAttribute("aria-current");
  });

  it("marks only the longest matching section active", async () => {
    route.pathname = "/watchdog/reports";
    await renderLayout();

    const nav = screen.getByRole("navigation", { name: "Primary" });
    expect(within(nav).getByRole("link", { name: "Reports" })).toHaveAttribute(
      "aria-current",
      "page"
    );
    expect(
      within(nav).getByRole("link", { name: "Watchdog" })
    ).not.toHaveAttribute("aria-current");
  });

  it("renders the footer after the page content", async () => {
    await renderLayout();
    const content = screen.getByText("page content");
    const footer = screen.getByRole("contentinfo");
    expect(
      content.compareDocumentPosition(footer) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
  });

  it("changes the shell language and saves the timezone preference", async () => {
    await renderLayout();

    fireEvent.change(screen.getByLabelText("Language"), {
      target: { value: "es" },
    });
    expect(screen.getByRole("link", { name: "Contratos" })).toBeTruthy();

    fireEvent.change(screen.getByLabelText("Zona horaria"), {
      target: { value: "local" },
    });
    expect(localStorage.getItem("sorolens_locale")).toBe("es");
    expect(localStorage.getItem("sorolens_timezone")).toBe("local");
  });
});
