/**
 * Tests for apps/web/app/(app)/layout.tsx
 *
 * The layout wraps every dashboard page, so asserting on it covers them all.
 */

import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

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

describe("AppLayout", () => {
  afterEach(() => {
    cleanup();
  });

  async function renderLayout() {
    const { default: AppLayout } = await import("@/app/(app)/layout");
    return render(
      <AppLayout>
        <p>page content</p>
      </AppLayout>,
    );
  }

  it("renders the Stellar community line in the footer", async () => {
    await renderLayout();
    const footer = screen.getByRole("contentinfo");
    expect(footer.textContent).toContain(
      "Built for the Stellar developer community.",
    );
  });

  it("renders the footer after the page content", async () => {
    await renderLayout();
    const content = screen.getByText("page content");
    const footer = screen.getByRole("contentinfo");
    expect(
      content.compareDocumentPosition(footer) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });
});
