/**
 * Tests for apps/web/app/(app)/layout.tsx
 *
 * The layout wraps every dashboard page, so asserting on it covers them all.
 */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

vi.mock("@/components/NetworkSelector", () => ({
  NetworkSelector: () => <div data-testid="network-selector" />,
}));

vi.mock("@/components/ThemeToggle", () => ({
  ThemeToggle: () => <div data-testid="theme-toggle" />,
}));

// The layout is rendered standalone here (the root layout's ThemeProvider is
// not in the tree), and CmdkSearch consumes useTheme directly.
vi.mock("@/components/ThemeProvider", () => ({
  useTheme: () => ({ theme: "light", setTheme: vi.fn() }),
}));

// The layout renders Breadcrumbs (useParams/usePathname) and CmdkSearch
// (useRouter); without an app-router context Next throws
// "invariant expected app router to be mounted".
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    prefetch: vi.fn(),
  }),
  usePathname: () => "/",
  useParams: () => ({}),
  useSearchParams: () => new URLSearchParams(),
}));

describe("AppLayout", () => {
  beforeEach(() => localStorage.clear());

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
