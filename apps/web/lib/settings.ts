/**
 * Persisted user preferences for the `/settings` page.
 *
 * The API has no user-profile endpoint for theme/network/notification
 * preferences, so those live in `localStorage` under a single key and the UI
 * labels them as browser-only. API keys are NOT stored here: they are minted
 * and revoked through `/api/v1/me/api-keys` (issue #613). Every helper takes
 * an optional `Storage` argument so it can be unit-tested without a DOM.
 *
 * Theme is deliberately NOT stored here: `ThemeProvider` already owns the
 * `theme` key and applies it to `<html data-theme>`, so the theme card reads and
 * writes through that provider instead of duplicating the state.
 */

export const SETTINGS_STORAGE_KEY = "sorolens_settings";

export type NotificationChannel =
  "critical_alerts" | "health_changes" | "weekly_digest";

export interface NotificationChannelMeta {
  id: NotificationChannel;
  label: string;
  description: string;
}

export const NOTIFICATION_CHANNELS: readonly NotificationChannelMeta[] = [
  {
    id: "critical_alerts",
    label: "Critical alerts",
    description: "Email me when a monitored contract raises a Critical alert.",
  },
  {
    id: "health_changes",
    label: "Health changes",
    description: "Email me when a contract moves to Degraded or Unresponsive.",
  },
  {
    id: "weekly_digest",
    label: "Weekly digest",
    description: "A Monday summary of activity across my tracked contracts.",
  },
];

export interface NotificationPreferences {
  email: string;
  channels: Record<NotificationChannel, boolean>;
}

export interface UserSettings {
  /** Network pre-selected in the header selector on first load. */
  defaultNetwork: string;
  notifications: NotificationPreferences;
}

export const DEFAULT_SETTINGS: UserSettings = {
  // "all" (ALL_NETWORKS in lib/network.tsx) keeps the dashboard's existing
  // unfiltered default, so the app only switches network once a user has
  // explicitly saved a preference here.
  defaultNetwork: "all",
  notifications: {
    email: "",
    channels: {
      critical_alerts: true,
      health_changes: true,
      weekly_digest: false,
    },
  },
};

/** Deliberately permissive: catches typos without rejecting valid addresses. */
export function isValidEmail(value: string): boolean {
  const trimmed = value.trim();
  if (trimmed.length === 0 || trimmed.length > 254) return false;
  if (/\s/.test(trimmed)) return false;
  return /^[^@.]+(\.[^@.]+)*@[^@.]+(\.[^@.]+)+$/.test(trimmed);
}

function coerceSettings(raw: unknown): UserSettings {
  if (raw === null || typeof raw !== "object") return DEFAULT_SETTINGS;
  const value = raw as Partial<UserSettings>;
  const channels = (
    value.notifications as Partial<NotificationPreferences> | undefined
  )?.channels;

  // Merge over the defaults so a blob written by an older build (or a partial
  // write) can never leave the page reading `undefined`.
  return {
    defaultNetwork:
      typeof value.defaultNetwork === "string" && value.defaultNetwork
        ? value.defaultNetwork
        : DEFAULT_SETTINGS.defaultNetwork,
    notifications: {
      email:
        typeof value.notifications?.email === "string"
          ? value.notifications.email
          : DEFAULT_SETTINGS.notifications.email,
      channels: {
        critical_alerts:
          channels?.critical_alerts ??
          DEFAULT_SETTINGS.notifications.channels.critical_alerts,
        health_changes:
          channels?.health_changes ??
          DEFAULT_SETTINGS.notifications.channels.health_changes,
        weekly_digest:
          channels?.weekly_digest ??
          DEFAULT_SETTINGS.notifications.channels.weekly_digest,
      },
    },
    // API keys are server-issued and live in component state (issue #613).
    // Any apiKey/apiKeyCreatedAt left in an older blob is dropped here so a
    // fabricated plaintext token can never survive a reload.
  };
}

function resolveStorage(storage?: Storage): Storage | null {
  if (storage) return storage;
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage;
  } catch {
    // Storage can throw when cookies are blocked; fall back to defaults.
    return null;
  }
}

/** Reads stored preferences, always returning a complete settings object. */
export function loadSettings(storage?: Storage): UserSettings {
  const store = resolveStorage(storage);
  if (!store) return DEFAULT_SETTINGS;
  try {
    const raw = store.getItem(SETTINGS_STORAGE_KEY);
    if (!raw) return DEFAULT_SETTINGS;
    return coerceSettings(JSON.parse(raw));
  } catch {
    return DEFAULT_SETTINGS;
  }
}

/** Persists preferences. Returns false when storage is unavailable or full. */
export function saveSettings(
  settings: UserSettings,
  storage?: Storage
): boolean {
  const store = resolveStorage(storage);
  if (!store) return false;
  try {
    store.setItem(SETTINGS_STORAGE_KEY, JSON.stringify(settings));
    return true;
  } catch {
    return false;
  }
}

export function clearSettings(storage?: Storage): void {
  const store = resolveStorage(storage);
  if (!store) return;
  try {
    store.removeItem(SETTINGS_STORAGE_KEY);
  } catch {
    // Nothing to do: the caller re-renders from defaults either way.
  }
}
