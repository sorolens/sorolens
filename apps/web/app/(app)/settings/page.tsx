"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiKeySection } from "@/components/settings/ApiKeySection";
import { NetworkSection } from "@/components/settings/NetworkSection";
import { NotificationsSection } from "@/components/settings/NotificationsSection";
import type { SaveState } from "@/components/settings/SettingsCard";
import { ThemeSection } from "@/components/settings/ThemeSection";
import { useNetwork } from "@/lib/network";
import {
  DEFAULT_SETTINGS,
  isValidEmail,
  loadSettings,
  saveSettings,
  type UserSettings,
} from "@/lib/settings";

type SectionId = "theme" | "network" | "notifications" | "apiKey";

/**
 * `/settings` — one place to configure theme, default network, notification
 * preferences, and an API key.
 *
 * The page owns the persisted value (`saved`) and the working copy (`draft`),
 * so every card reads from the same source of truth and only `Save` writes to
 * storage. Theme is the one exception: it is owned by `ThemeProvider`, which
 * both applies and persists it, so that card writes through to the provider.
 */
export default function SettingsPage() {
  const { setNetwork } = useNetwork();
  const [saved, setSaved] = useState<UserSettings>(DEFAULT_SETTINGS);
  const [draft, setDraft] = useState<UserSettings>(DEFAULT_SETTINGS);
  const [loaded, setLoaded] = useState(false);
  const [status, setStatus] = useState<Partial<Record<SectionId, SaveState>>>(
    {}
  );

  // Read stored preferences after mount so server and client markup match.
  // The cards render only once this has run: showing the defaults first would
  // both flash the wrong values and let a pre-hydration edit be clobbered.
  useEffect(() => {
    const stored = loadSettings();
    setSaved(stored);
    setDraft(stored);
    setLoaded(true);
  }, []);

  const commit = useCallback((next: UserSettings, section: SectionId) => {
    const ok = saveSettings(next);
    setSaved(next);
    setDraft(next);
    setStatus((current) => ({
      ...current,
      [section]: ok ? "saved" : "error",
    }));
    return ok;
  }, []);

  const email = draft.notifications.email.trim();
  const emailError =
    email.length > 0 && !isValidEmail(email)
      ? "Enter a valid email address, or leave it blank to turn email notifications off."
      : null;

  const apiKeyDirty =
    draft.apiKey !== saved.apiKey ||
    draft.apiKeyCreatedAt !== saved.apiKeyCreatedAt;

  return (
    <div className="mx-auto max-w-3xl">
      <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">
        Settings
      </h1>
      <p className="mt-2 text-sm text-[var(--color-text-secondary)]">
        Configure how Sorolens looks, which network it defaults to, and how it
        reaches you. Preferences are stored in this browser.
      </p>

      <div className="mt-6 space-y-6">
        {loaded ? (
          <>
            <ThemeSection
              status={status.theme ?? "idle"}
              onSave={() => {
                setStatus((current) => ({ ...current, theme: "saved" }));
                return true;
              }}
            />

            <NetworkSection
              value={draft.defaultNetwork}
              status={status.network ?? "idle"}
              onChange={(defaultNetwork) =>
                setDraft((current) => ({ ...current, defaultNetwork }))
              }
              onSave={() => {
                const ok = commit(draft, "network");
                if (ok) setNetwork(draft.defaultNetwork);
                return ok;
              }}
            />

            <NotificationsSection
              preferences={draft.notifications}
              error={emailError}
              status={status.notifications ?? "idle"}
              onChange={(notifications) =>
                setDraft((current) => ({ ...current, notifications }))
              }
              onSave={() => {
                if (emailError) return false;
                return commit(
                  {
                    ...draft,
                    notifications: { ...draft.notifications, email },
                  },
                  "notifications"
                );
              }}
            />

            <ApiKeySection
              apiKey={draft.apiKey}
              apiKeyCreatedAt={draft.apiKeyCreatedAt}
              dirty={apiKeyDirty}
              status={status.apiKey ?? "idle"}
              onChange={(apiKey, apiKeyCreatedAt) =>
                setDraft((current) => ({ ...current, apiKey, apiKeyCreatedAt }))
              }
              onSave={() => commit(draft, "apiKey")}
            />
          </>
        ) : (
          // Placeholder while stored preferences load; never a flash of the
          // default values, which would be wrong for a returning user.
          Array.from({ length: 4 }).map((_, index) => (
            <div
              key={index}
              data-testid="settings-loading"
              className="h-48 animate-pulse rounded-lg bg-[var(--color-bg-card)]"
            />
          ))
        )}
      </div>
    </div>
  );
}
