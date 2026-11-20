/**
 * Tests for apps/web/app/(app)/settings/page.tsx
 *
 * vitest + @testing-library/react + jsdom. The theme provider and the network
 * context are mocked so each section can be driven without mounting the whole
 * app shell; `localStorage` is the real jsdom implementation, which is what the
 * page actually persists to. API-key and identity calls are mocked at the
 * client boundary so the test never reaches the network.
 */

import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const setTheme = vi.fn();
const setNetwork = vi.fn();
const listMyApiKeys = vi.fn();
const createMyApiKey = vi.fn();
const revokeMyApiKey = vi.fn();
const ensureSession = vi.fn();

vi.mock("@/components/ThemeProvider", () => ({
  useTheme: () => ({ theme: "system", setTheme }),
  ThemeProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("@/lib/network", () => ({
  ALL_NETWORKS: "all",
  NETWORKS: ["all", "testnet", "mainnet", "futurenet"],
  networkFilter: (network: string) => (network === "all" ? undefined : network),
  useNetwork: () => ({ network: "all", setNetwork }),
}));

vi.mock("@/lib/apiKeys", () => ({
  listMyApiKeys: () => listMyApiKeys(),
  createMyApiKey: () => createMyApiKey(),
  revokeMyApiKey: (id: string) => revokeMyApiKey(id),
}));

vi.mock("@/lib/user", () => ({
  ensureSession: () => ensureSession(),
}));

import { SETTINGS_STORAGE_KEY } from "@/lib/settings";

const KEY = {
  id: "key-1",
  name: "Dashboard key",
  key_prefix: "sl_abcdefgh",
  scopes: ["read:contracts"],
  user_id: "sess_test",
  created_at: "2026-10-01T00:00:00.000Z",
  last_used_at: null,
  revoked_at: null,
  rotated_at: null,
};

async function renderPage() {
  const { default: SettingsPage } = await import("@/app/(app)/settings/page");
  render(<SettingsPage />);
  // The page reads stored preferences in an effect after mount.
  await waitFor(() =>
    expect(screen.getByTestId("settings-theme")).toBeTruthy()
  );
}

function cardSave(testId: string) {
  return within(screen.getByTestId(testId)).getByRole("button", {
    name: "Save",
  });
}

// The section is also named "Default network" via aria-labelledby, so scope to
// the card before asking for the labelled control (mirrors settings.spec.ts).
function networkSelect() {
  return within(screen.getByTestId("settings-network")).getByLabelText(
    "Default network"
  );
}

describe("SettingsPage", () => {
  beforeEach(() => {
    localStorage.clear();
    setTheme.mockClear();
    setNetwork.mockClear();
    listMyApiKeys.mockReset();
    listMyApiKeys.mockResolvedValue([]);
    createMyApiKey.mockReset();
    revokeMyApiKey.mockReset();
    ensureSession.mockReset();
    ensureSession.mockResolvedValue({
      user_id: "sess_test",
      token: "sls_token",
      expires_at: "2099-01-01T00:00:00.000Z",
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("renders every settings section", async () => {
    await renderPage();

    expect(screen.getByRole("heading", { name: "Settings" })).toBeTruthy();
    expect(screen.getByTestId("settings-theme")).toBeTruthy();
    expect(screen.getByTestId("settings-network")).toBeTruthy();
    expect(screen.getByTestId("settings-notifications")).toBeTruthy();
    expect(screen.getByTestId("settings-identity")).toBeTruthy();
    expect(screen.getByTestId("settings-api-key")).toBeTruthy();
  });

  it("loads stored preferences", async () => {
    localStorage.setItem(
      SETTINGS_STORAGE_KEY,
      JSON.stringify({
        defaultNetwork: "mainnet",
        notifications: {
          email: "dev@example.com",
          channels: {
            critical_alerts: true,
            health_changes: false,
            weekly_digest: true,
          },
        },
      })
    );

    await renderPage();

    expect((networkSelect() as HTMLSelectElement).value).toBe("mainnet");
    expect(
      (screen.getByLabelText("Email address") as HTMLInputElement).value
    ).toBe("dev@example.com");
    expect(
      (screen.getByLabelText("Weekly digest") as HTMLInputElement).checked
    ).toBe(true);
    expect(
      (screen.getByLabelText("Health changes") as HTMLInputElement).checked
    ).toBe(false);
  });

  it("saves the default network and switches the header selector", async () => {
    await renderPage();

    fireEvent.change(networkSelect(), {
      target: { value: "futurenet" },
    });
    fireEvent.click(cardSave("settings-network"));

    await waitFor(() =>
      expect(screen.getByTestId("settings-network-status")).toBeTruthy()
    );
    expect(setNetwork).toHaveBeenCalledWith("futurenet");
    expect(
      JSON.parse(localStorage.getItem(SETTINGS_STORAGE_KEY) as string)
        .defaultNetwork
    ).toBe("futurenet");
  });

  it("refuses to save a malformed notification email", async () => {
    await renderPage();

    fireEvent.change(screen.getByLabelText("Email address"), {
      target: { value: "not-an-email" },
    });

    expect(screen.getByTestId("settings-notifications-error")).toBeTruthy();
    fireEvent.click(cardSave("settings-notifications"));

    // Nothing persisted, and no success status appears.
    expect(screen.queryByTestId("settings-notifications-status")).toBeNull();
    expect(localStorage.getItem(SETTINGS_STORAGE_KEY)).toBeNull();
  });

  it("saves notification preferences", async () => {
    await renderPage();

    fireEvent.change(screen.getByLabelText("Email address"), {
      target: { value: "dev@example.com" },
    });
    fireEvent.click(screen.getByLabelText("Weekly digest"));
    fireEvent.click(cardSave("settings-notifications"));

    await waitFor(() =>
      expect(screen.getByTestId("settings-notifications-status")).toBeTruthy()
    );
    const stored = JSON.parse(
      localStorage.getItem(SETTINGS_STORAGE_KEY) as string
    );
    expect(stored.notifications.email).toBe("dev@example.com");
    expect(stored.notifications.channels.weekly_digest).toBe(true);
  });

  it("shows the verified user ID with a copy button", async () => {
    await renderPage();

    await waitFor(() =>
      expect(screen.getByTestId("settings-user-id").textContent).toBe(
        "sess_test"
      )
    );
    expect(screen.getByTestId("settings-user-id-copy")).toBeTruthy();
  });

  it("creates a real key through the API and never stores the plaintext", async () => {
    const created = { ...KEY, key: "sl_plaintext_secret" };
    createMyApiKey.mockResolvedValueOnce(created);

    await renderPage();
    expect(
      screen.getByText("No API key yet. Create one to authenticate requests.")
    ).toBeTruthy();

    fireEvent.click(
      within(screen.getByTestId("settings-api-key")).getByRole("button", {
        name: "Generate key",
      })
    );

    await waitFor(() =>
      expect(screen.getByTestId("settings-api-key-value").textContent).toBe(
        "sl_plaintext_secret"
      )
    );
    expect(createMyApiKey).toHaveBeenCalledTimes(1);
    // The plaintext is display-once component state, never persisted.
    const stored = localStorage.getItem(SETTINGS_STORAGE_KEY);
    expect(stored === null || !stored.includes("sl_plaintext_secret")).toBe(
      true
    );
  });

  it("shows the server-issued prefix and revokes through the API", async () => {
    listMyApiKeys.mockResolvedValueOnce([KEY]);
    revokeMyApiKey.mockResolvedValueOnce(undefined);

    await renderPage();

    await waitFor(() =>
      expect(screen.getByTestId("settings-api-key-row").textContent).toContain(
        "sl_abcdefgh"
      )
    );
    fireEvent.click(
      within(screen.getByTestId("settings-api-key-row")).getByRole("button", {
        name: "Revoke",
      })
    );

    const confirmButton = await screen.findByRole("button", {
      name: "Revoke key",
    });
    fireEvent.click(confirmButton);

    await waitFor(() => expect(revokeMyApiKey).toHaveBeenCalledWith("key-1"));
    expect(screen.queryByTestId("settings-api-key-row")).toBeNull();
  });

  it("applies a theme change through the provider", async () => {
    await renderPage();

    fireEvent.click(screen.getByLabelText("Dark"));
    expect(setTheme).toHaveBeenCalledWith("dark");

    fireEvent.click(cardSave("settings-theme"));
    expect(screen.getByTestId("settings-theme-status")).toBeTruthy();
  });
});
