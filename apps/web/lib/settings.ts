/**
 * Persisted user preferences for the `/settings` page.
 *
 * The API has no user-profile endpoint yet, so preferences live in
 * `localStorage` under a single key. Every helper takes an optional `Storage`
 * argument so it can be unit-tested without a DOM.
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
  /**
   * Server-issued key reference (issue #613). The API keeps only a hash of
   * the token and returns the plaintext once at creation, so the plaintext is
   * never persisted here — just the id and non-secret prefix needed to show
   * and revoke the key. Null when no key exists.
   */
  apiKeyId: string | null;
  /** Non-secret key prefix echoed by the API, e.g. "sl_ab12cd3". */
  apiKeyPrefix: string | null;
  apiKeyCreatedAt: string | null;
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
  apiKeyId: null,
  apiKeyPrefix: null,
  apiKeyCreatedAt: null,
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
    // Issue #613: a pre-#613 blob stored a plaintext `apiKey` string that was
    // never a real credential (nothing on the server knew about it). Reading
    // it into the new shape drops the fake key entirely rather than displaying
    // it as if it authenticated anything, and the next save persists the
    // corrected shape. Only the server-issued id and non-secret prefix survive.
    apiKeyId:
      typeof value.apiKeyId === "string" && value.apiKeyId
        ? value.apiKeyId
        : null,
    apiKeyPrefix:
      typeof value.apiKeyPrefix === "string" && value.apiKeyPrefix
        ? value.apiKeyPrefix
        : null,
    apiKeyCreatedAt:
      typeof value.apiKeyCreatedAt === "string" ? value.apiKeyCreatedAt : null,
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
