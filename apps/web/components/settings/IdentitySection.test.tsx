/**
 * Tests for apps/web/components/settings/IdentitySection.tsx (issue #765).
 *
 * The card reads the user ID from the verified workspace session, so a failed
 * `POST /api/v1/auth/session` must surface as an error rather than a blank
 * identity the user might copy. `@/lib/user` is mocked: no real network.
 */

import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { IdentitySection } from "./IdentitySection";

const mockEnsureSession = vi.fn();

vi.mock("@/lib/user", () => ({
  ensureSession: () => mockEnsureSession(),
}));

function stubClipboard(writeText: (value: string) => Promise<void>) {
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn(writeText) },
    configurable: true,
  });
  return navigator.clipboard.writeText as unknown as ReturnType<typeof vi.fn>;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockEnsureSession.mockResolvedValue({
    user_id: "user-9f2c",
    token: "sls_token",
    expires_at: new Date(Date.now() + 3_600_000).toISOString(),
  });
});

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(navigator, "clipboard");
});

describe("IdentitySection", () => {
  it("shows the session user ID once it resolves", async () => {
    render(<IdentitySection />);

    expect(screen.getByTestId("settings-user-id")).toHaveTextContent(
      "Loading…"
    );
    expect(await screen.findByText("user-9f2c")).toBeVisible();
    expect(screen.getByTestId("settings-user-id-copy")).toBeEnabled();
  });

  it("reports a session failure and keeps copying disabled", async () => {
    mockEnsureSession.mockRejectedValue(
      new Error("Workspace sessions are not configured on this API.")
    );
    render(<IdentitySection />);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(
      "Workspace sessions are not configured on this API."
    );
    expect(screen.getByTestId("settings-user-id")).toHaveTextContent("—");
    expect(screen.getByTestId("settings-user-id-copy")).toBeDisabled();
  });

  it("falls back to a generic message for a non-Error rejection", async () => {
    mockEnsureSession.mockRejectedValue("nope");
    render(<IdentitySection />);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Could not load your user ID."
    );
  });

  it("copies the user ID to the clipboard", async () => {
    const writeText = stubClipboard(async () => {});
    render(<IdentitySection />);
    await screen.findByText("user-9f2c");

    fireEvent.click(screen.getByTestId("settings-user-id-copy"));

    await waitFor(() =>
      expect(screen.getByTestId("settings-user-id-copy")).toHaveTextContent(
        "Copied"
      )
    );
    expect(writeText).toHaveBeenCalledWith("user-9f2c");
  });

  it("reports a clipboard failure instead of claiming it copied", async () => {
    stubClipboard(async () => {
      throw new Error("denied");
    });
    render(<IdentitySection />);
    await screen.findByText("user-9f2c");

    fireEvent.click(screen.getByTestId("settings-user-id-copy"));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Could not copy to the clipboard."
    );
    expect(screen.getByTestId("settings-user-id-copy")).toHaveTextContent(
      "Copy"
    );
  });
});
