/**
 * Tests for apps/web/components/settings/ExtensionSection.tsx (issue #765).
 *
 * The extension gets a scoped, revocable key instead of a raw user ID. The
 * card must request exactly the documented scope and name, show the token once
 * and report a failure without pretending a key exists.
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
import { ExtensionSection } from "./ExtensionSection";
import {
  EXTENSION_INSTALL_URL,
  EXTENSION_KEY_NAME,
  EXTENSION_KEY_SCOPES,
  EXTENSION_RELEASES_URL,
} from "@/lib/extension";

const mockCreateMyApiKey = vi.fn();

vi.mock("@/lib/apiKeys", () => ({
  createMyApiKey: (...args: unknown[]) => mockCreateMyApiKey(...args),
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
  mockCreateMyApiKey.mockResolvedValue({
    id: "key-ext",
    name: EXTENSION_KEY_NAME,
    key_prefix: "slo_ext",
    scopes: [...EXTENSION_KEY_SCOPES],
    user_id: "user-1",
    created_at: "2026-09-01T00:00:00Z",
    last_used_at: null,
    revoked_at: null,
    rotated_at: null,
    key: "slo_extension_token",
  });
});

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(navigator, "clipboard");
});

describe("ExtensionSection", () => {
  it("links to the install and release instructions", () => {
    render(<ExtensionSection />);

    expect(
      screen.getByRole("link", { name: "Install instructions" })
    ).toHaveAttribute("href", EXTENSION_INSTALL_URL);
    expect(
      screen.getByRole("link", { name: "Release builds" })
    ).toHaveAttribute("href", EXTENSION_RELEASES_URL);
  });

  it("documents the scopes the key is given", () => {
    render(<ExtensionSection />);

    expect(
      screen.getByText(new RegExp(EXTENSION_KEY_SCOPES.join(" and ")))
    ).toBeVisible();
  });

  it("creates a key with the extension name and scopes", async () => {
    render(<ExtensionSection />);

    fireEvent.click(
      screen.getByRole("button", { name: "Create extension key" })
    );

    const key = await screen.findByTestId("settings-extension-key");
    expect(key).toHaveTextContent("slo_extension_token");
    expect(mockCreateMyApiKey).toHaveBeenCalledWith(EXTENSION_KEY_NAME, [
      ...EXTENSION_KEY_SCOPES,
    ]);
    expect(
      screen.getByRole("button", { name: "Create another key" })
    ).toBeEnabled();
  });

  it("reports a failed creation and shows no key", async () => {
    mockCreateMyApiKey.mockRejectedValue(new Error("Forbidden"));
    render(<ExtensionSection />);

    fireEvent.click(
      screen.getByRole("button", { name: "Create extension key" })
    );

    expect(
      await screen.findByTestId("settings-extension-error")
    ).toHaveTextContent("Forbidden");
    expect(screen.queryByTestId("settings-extension-key")).toBeNull();
    expect(
      screen.getByRole("button", { name: "Create extension key" })
    ).toBeEnabled();
  });

  it("falls back to a generic message for a non-Error rejection", async () => {
    mockCreateMyApiKey.mockRejectedValue(undefined);
    render(<ExtensionSection />);

    fireEvent.click(
      screen.getByRole("button", { name: "Create extension key" })
    );

    expect(
      await screen.findByTestId("settings-extension-error")
    ).toHaveTextContent("Could not create an extension key.");
  });

  it("copies the created key", async () => {
    const writeText = stubClipboard(async () => {});
    render(<ExtensionSection />);
    fireEvent.click(
      screen.getByRole("button", { name: "Create extension key" })
    );
    await screen.findByTestId("settings-extension-key");

    fireEvent.click(screen.getByTestId("settings-extension-copy"));

    await waitFor(() =>
      expect(screen.getByTestId("settings-extension-copy")).toHaveTextContent(
        "Copied"
      )
    );
    expect(writeText).toHaveBeenCalledWith("slo_extension_token");
  });

  it("reports a clipboard failure", async () => {
    stubClipboard(async () => {
      throw new Error("denied");
    });
    render(<ExtensionSection />);
    fireEvent.click(
      screen.getByRole("button", { name: "Create extension key" })
    );
    await screen.findByTestId("settings-extension-key");

    fireEvent.click(screen.getByTestId("settings-extension-copy"));

    expect(
      await screen.findByTestId("settings-extension-error")
    ).toHaveTextContent("Could not copy to the clipboard.");
  });
});
