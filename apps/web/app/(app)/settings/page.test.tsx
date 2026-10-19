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
import { SESSION_STORAGE_KEY } from "@/lib/user";

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
    vi.unstubAllGlobals();
  });

  it("renders all sections including the API-issued identity card", async () => {
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
    // Issue #613: a stored pre-#613 plaintext `apiKey` was a fabricated local
    // string, so coerceSettings drops it; the card shows no key.
    expect(screen.getByTestId("settings-api-key-value").textContent).toBe("—");
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

  it("generates a real key through the API, persists only id and prefix, then revokes", async () => {
    // Issue #613: the card must talk to POST/DELETE /api/v1/api-keys with the
    // session bearer, keep the one-time plaintext in memory only, and persist
    // nothing but the server-issued id and non-secret prefix.
    localStorage.setItem(
      SESSION_STORAGE_KEY,
      JSON.stringify({
        user_id: "u_42",
        token: "sls_sig",
        expires_at: new Date(Date.now() + 3600_000).toISOString(),
      })
    );
    const plaintext = "sl_ab12cd3_7f4bd2a1e5c9330f";
    const fetchMock = vi.fn(
      async (
        input: string,
        init?: { method?: string; headers?: Record<string, string> }
      ) => {
        if (init?.method === "POST") {
          return new Response(
            JSON.stringify({
              id: "key_1",
              name: "Sorolens settings key",
              key_prefix: "sl_ab12cd3",
              created_at: "2026-10-01T00:00:00.000Z",
              key: plaintext,
            }),
            { status: 201, headers: { "Content-Type": "application/json" } }
          );
        }
        if (init?.method === "DELETE") {
          return new Response(null, { status: 204 });
        }
        throw new Error(`unexpected fetch: ${init?.method} ${input}`);
      }
    );
    vi.stubGlobal("fetch", fetchMock);

    await renderPage();

    expect(screen.getByTestId("settings-api-key-value").textContent).toBe("—");

    fireEvent.click(
      within(screen.getByTestId("settings-api-key")).getByRole("button", {
        name: "Generate key",
      })
    );
    await waitFor(() =>
      expect(screen.getByTestId("settings-api-key-value").textContent).toBe(
        plaintext
      )
    );
    const [postUrl, postInit] = fetchMock.mock.calls[0]!;
    expect(String(postUrl)).toContain("/api/v1/api-keys");
    expect(postInit?.headers?.Authorization).toBe("Bearer sls_sig");

    fireEvent.click(cardSave("settings-api-key"));
    await waitFor(() =>
      expect(screen.getByTestId("settings-api-key-status")).toBeTruthy()
    );
    const persisted = localStorage.getItem(SETTINGS_STORAGE_KEY) as string;
    const stored = JSON.parse(persisted);
    expect(stored.apiKeyId).toBe("key_1");
    expect(stored.apiKeyPrefix).toBe("sl_ab12cd3");
    // The one-time plaintext must never reach storage (issue #613).
    expect(persisted).not.toContain(plaintext);

    fireEvent.click(
      within(screen.getByTestId("settings-api-key")).getByRole("button", {
        name: "Revoke",
      })
    );
    expect(fetchMock.mock.calls[1]?.[1]?.method).toBe("DELETE");
    // Let the DELETE settle before saving: the card only clears the draft
    // after the API confirms, same as a real operator's click order.
    await waitFor(() =>
      expect(screen.getByTestId("settings-api-key-value").textContent).toBe("—")
    );
    fireEvent.click(cardSave("settings-api-key"));

    await waitFor(() =>
      expect(screen.getByTestId("settings-api-key-status")).toBeTruthy()
    );
    expect(
      JSON.parse(localStorage.getItem(SETTINGS_STORAGE_KEY) as string).apiKeyId
    ).toBeNull();

    vi.unstubAllGlobals();
  });

  it("applies a theme change through the provider", async () => {
    await renderPage();

    fireEvent.click(screen.getByLabelText("Dark"));
    expect(setTheme).toHaveBeenCalledWith("dark");

    fireEvent.click(cardSave("settings-theme"));
    expect(screen.getByTestId("settings-theme-status")).toBeTruthy();
  });
});
