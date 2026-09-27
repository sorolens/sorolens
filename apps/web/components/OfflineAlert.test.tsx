/**
 * Tests for apps/web/components/OfflineAlert.tsx
 *
 * We use vitest + @testing-library/react + jsdom.
 * next/link is mocked to a plain <a> so we don't need the Next.js runtime.
 */

import type { AnchorHTMLAttributes, ReactNode } from "react";
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { QueuedAlert } from "@/lib/alertQueue";
import { OfflineAlertPanel } from "./OfflineAlert";

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...rest
  }: {
    children: ReactNode;
    href: string;
  } & AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

const LONG_CONTRACT_ID =
  "CAVRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C3VRQGH5C33";

const ALERT: QueuedAlert = {
  id: "alert-1",
  contractId: LONG_CONTRACT_ID,
  severity: "Warning",
  message: "Storage TTL is running low",
  timestamp: "2026-01-01T00:00:00Z",
  enqueuedAt: "2026-01-01T00:00:00Z",
  dismissed: false,
};

afterEach(() => {
  cleanup();
});

describe("OfflineAlertPanel", () => {
  it("shows the full contract id in a tooltip on the truncated id", () => {
    render(
      <OfflineAlertPanel
        alerts={[ALERT]}
        onDismiss={() => {}}
        onDismissAll={() => {}}
      />
    );

    expect(screen.getByText(LONG_CONTRACT_ID)).toHaveAttribute(
      "title",
      LONG_CONTRACT_ID
    );
  });
});
