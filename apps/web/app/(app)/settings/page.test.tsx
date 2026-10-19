/**
 * Tests for apps/web/app/(app)/settings/page.tsx
 *
 * vitest + @testing-library/react + jsdom. The theme provider and the network
 * context are mocked so each section can be driven without mounting the whole
 * app shell; `localStorage` is the real jsdom implementation, which is what the
 * page actually persists to.
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

import { SETTINGS_STORAGE_KEY } from "@/lib/settings";

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
  });

  afterEach(() => {
    cleanup();
  });

  it("renders all four sections", async () => {
    await renderPage();

    expect(screen.getByRole("heading", { name: "Settings" })).toBeTruthy();
    expect(screen.getByTestId("settings-theme")).toBeTruthy();
    expect(screen.getByTestId("settings-network")).toBeTruthy();
    expect(screen.getByTestId("settings-notifications")).toBeTruthy();
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
        apiKey: "sl_live_stored",
        apiKeyCreatedAt: "2026-09-25T00:00:00.000Z",
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
    expect(screen.getByTestId("settings-api-key-value").textContent).toContain(
      "sl_live_"
    );
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

  it("generates an API key, persists it, then revokes it", async () => {
    await renderPage();

    expect(screen.getByTestId("settings-api-key-value").textContent).toBe("—");

    fireEvent.click(
      within(screen.getByTestId("settings-api-key")).getByRole("button", {
        name: "Generate key",
      })
    );
    const generated = screen
      .getByTestId("settings-api-key-value")
      .textContent?.trim() as string;
    expect(generated.startsWith("sl_live_")).toBe(true);

    fireEvent.click(cardSave("settings-api-key"));
    await waitFor(() =>
      expect(screen.getByTestId("settings-api-key-status")).toBeTruthy()
    );
    expect(
      JSON.parse(localStorage.getItem(SETTINGS_STORAGE_KEY) as string).apiKey
    ).toBe(generated);

    fireEvent.click(
      within(screen.getByTestId("settings-api-key")).getByRole("button", {
        name: "Revoke",
      })
    );
    fireEvent.click(cardSave("settings-api-key"));

    await waitFor(() =>
      expect(screen.getByTestId("settings-api-key-value").textContent).toBe("—")
    );
    expect(
      JSON.parse(localStorage.getItem(SETTINGS_STORAGE_KEY) as string).apiKey
    ).toBeNull();
  });

  it("applies a theme change through the provider", async () => {
    await renderPage();

    fireEvent.click(screen.getByLabelText("Dark"));
    expect(setTheme).toHaveBeenCalledWith("dark");

    fireEvent.click(cardSave("settings-theme"));
    expect(screen.getByTestId("settings-theme-status")).toBeTruthy();
  });
});
