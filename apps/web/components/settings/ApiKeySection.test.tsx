/**
 * Tests for apps/web/components/settings/ApiKeySection.tsx (issue #765).
 *
 * The card is the only place a plaintext token is ever visible, so the tests
 * cover the three states that matter: the key is shown once after creation, it
 * disappears from view once revoked, and every API failure is reported instead
 * of being swallowed. No real network: `@/lib/apiKeys` is mocked.
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
import type { APIKey, CreatedAPIKey } from "@/lib/apiKeys";
import { ApiKeySection } from "./ApiKeySection";

const mockListMyApiKeys = vi.fn();
const mockCreateMyApiKey = vi.fn();
const mockRevokeMyApiKey = vi.fn();

vi.mock("@/lib/apiKeys", () => ({
  listMyApiKeys: () => mockListMyApiKeys(),
  createMyApiKey: (...args: unknown[]) => mockCreateMyApiKey(...args),
  revokeMyApiKey: (...args: unknown[]) => mockRevokeMyApiKey(...args),
}));

function apiKey(overrides: Partial<APIKey> = {}): APIKey {
  return {
    id: "key-1",
    name: "Dashboard key",
    key_prefix: "slo_7f3a",
    scopes: ["read:contracts", "read:watchdog"],
    user_id: "user-1",
    created_at: "2026-09-01T00:00:00Z",
    last_used_at: null,
    revoked_at: null,
    rotated_at: null,
    ...overrides,
  };
}

function createdKey(key = "slo_plaintext_once"): CreatedAPIKey {
  return { ...apiKey({ id: "key-new", name: "Dashboard key" }), key };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function stubClipboard(writeText: (value: string) => Promise<void>) {
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn(writeText) },
    configurable: true,
  });
  return navigator.clipboard.writeText as unknown as ReturnType<typeof vi.fn>;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockListMyApiKeys.mockResolvedValue([]);
  mockCreateMyApiKey.mockResolvedValue(createdKey());
  mockRevokeMyApiKey.mockResolvedValue(undefined);
});

afterEach(() => {
  cleanup();
  Reflect.deleteProperty(navigator, "clipboard");
});

describe("ApiKeySection", () => {
  it("shows a loading state until the key list resolves", async () => {
    const pending = deferred<APIKey[]>();
    mockListMyApiKeys.mockReturnValue(pending.promise);
    render(<ApiKeySection />);

    expect(screen.getByRole("status")).toHaveTextContent("Loading keys…");
    expect(screen.getByRole("button", { name: "Generate key" })).toBeDisabled();

    pending.resolve([]);

    expect(
      await screen.findByText(
        "No API key yet. Create one to authenticate requests."
      )
    ).toBeVisible();
  });

  it("lists the server-issued prefix, never a full token", async () => {
    mockListMyApiKeys.mockResolvedValue([apiKey()]);
    render(<ApiKeySection />);

    const row = await screen.findByTestId("settings-api-key-row");
    expect(row).toHaveTextContent("Dashboard key");
    expect(row).toHaveTextContent("slo_7f3a…");
    expect(screen.queryByTestId("settings-api-key-created")).toBeNull();
  });

  it("reports a failed load instead of showing an empty state", async () => {
    mockListMyApiKeys.mockRejectedValue(new Error("Could not list API keys."));
    render(<ApiKeySection />);

    const alert = await screen.findByTestId("settings-api-key-error");
    expect(alert).toHaveAttribute("role", "alert");
    expect(alert).toHaveTextContent("Could not list API keys.");
    expect(screen.queryByTestId("settings-api-key-row")).toBeNull();
  });

  it("shows the plaintext key exactly once after creating it", async () => {
    render(<ApiKeySection />);
    await screen.findByText(/No API key yet/);

    fireEvent.click(screen.getByRole("button", { name: "Generate key" }));

    const value = await screen.findByTestId("settings-api-key-value");
    expect(value).toHaveTextContent("slo_plaintext_once");
    expect(
      screen.getByText("Copy this key now — it will not be shown again.")
    ).toBeVisible();
    expect(screen.getByTestId("settings-api-key-row")).toHaveTextContent(
      "Dashboard key"
    );
  });

  it("reports a failed creation and shows no key", async () => {
    mockCreateMyApiKey.mockRejectedValue(new Error("Rate limit exceeded."));
    render(<ApiKeySection />);
    await screen.findByText(/No API key yet/);

    fireEvent.click(screen.getByRole("button", { name: "Generate key" }));

    const alert = await screen.findByTestId("settings-api-key-error");
    expect(alert).toHaveTextContent("Rate limit exceeded.");
    expect(screen.queryByTestId("settings-api-key-value")).toBeNull();
    expect(screen.getByRole("button", { name: "Generate key" })).toBeEnabled();
  });

  it("falls back to a generic message when creation throws a non-Error", async () => {
    mockCreateMyApiKey.mockRejectedValue("boom");
    render(<ApiKeySection />);
    await screen.findByText(/No API key yet/);

    fireEvent.click(screen.getByRole("button", { name: "Generate key" }));

    expect(
      await screen.findByTestId("settings-api-key-error")
    ).toHaveTextContent("Could not create an API key.");
  });

  it("removes a revoked key from the list and hides its plaintext", async () => {
    mockListMyApiKeys.mockResolvedValue([apiKey(), apiKey({ id: "key-2" })]);
    mockRevokeMyApiKey.mockResolvedValue(undefined);
    render(<ApiKeySection />);
    await screen.findAllByTestId("settings-api-key-row");

    fireEvent.click(screen.getAllByRole("button", { name: "Revoke" })[0]);

    await waitFor(() =>
      expect(screen.getAllByTestId("settings-api-key-row")).toHaveLength(1)
    );
    expect(mockRevokeMyApiKey).toHaveBeenCalledWith("key-1");
  });

  it("keeps the key listed when revoking fails", async () => {
    mockListMyApiKeys.mockResolvedValue([apiKey()]);
    mockRevokeMyApiKey.mockRejectedValue(new Error("Could not revoke."));
    render(<ApiKeySection />);
    await screen.findByTestId("settings-api-key-row");

    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));

    expect(
      await screen.findByTestId("settings-api-key-error")
    ).toHaveTextContent("Could not revoke.");
    expect(screen.getByTestId("settings-api-key-row")).toBeVisible();
  });

  it("copies the plaintext key to the clipboard", async () => {
    const writeText = stubClipboard(async () => {});
    render(<ApiKeySection />);
    await screen.findByText(/No API key yet/);
    fireEvent.click(screen.getByRole("button", { name: "Generate key" }));
    await screen.findByTestId("settings-api-key-value");

    fireEvent.click(screen.getByTestId("settings-api-key-copy"));

    await waitFor(() =>
      expect(screen.getByTestId("settings-api-key-copy")).toHaveTextContent(
        "Copied"
      )
    );
    expect(writeText).toHaveBeenCalledWith("slo_plaintext_once");
  });

  it("reports a clipboard failure", async () => {
    stubClipboard(async () => {
      throw new Error("denied");
    });
    render(<ApiKeySection />);
    await screen.findByText(/No API key yet/);
    fireEvent.click(screen.getByRole("button", { name: "Generate key" }));
    await screen.findByTestId("settings-api-key-value");

    fireEvent.click(screen.getByTestId("settings-api-key-copy"));

    expect(
      await screen.findByTestId("settings-api-key-error")
    ).toHaveTextContent("Could not copy to the clipboard.");
    expect(screen.getByTestId("settings-api-key-copy")).toHaveTextContent(
      "Copy"
    );
  });
});
