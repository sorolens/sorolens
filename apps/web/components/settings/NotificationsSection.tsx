"use client";

import { SettingsCard, SettingsRow, type SaveState } from "./SettingsCard";
import {
  NOTIFICATION_CHANNELS,
  type NotificationPreferences,
} from "@/lib/settings";

/**
 * User-level notification preferences. Per-contract destinations (Slack,
 * Discord, PagerDuty, generic webhooks) are managed per contract on the
 * Watchdog page; this card only decides which emails a user receives.
 */
export function NotificationsSection({
  preferences,
  error,
  status,
  onChange,
  onSave,
}: {
  preferences: NotificationPreferences;
  error: string | null;
  status: SaveState;
  onChange: (preferences: NotificationPreferences) => void;
  onSave: () => boolean;
}) {
  return (
    <SettingsCard
      testId="settings-notifications"
      title="Notification preferences"
      description="Which emails Sorolens sends you about the contracts you track."
      status={status}
      onSave={onSave}
    >
      <SettingsRow
        htmlFor="settings-notification-email"
        title="Email address"
        description="Leave blank to turn email notifications off."
      >
        <input
          id="settings-notification-email"
          type="email"
          value={preferences.email}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? "settings-notification-error" : undefined}
          onChange={(event) =>
            onChange({ ...preferences, email: event.target.value })
          }
          placeholder="you@example.com"
          className="w-56 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-page)] px-2 py-1 text-sm text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none"
        />
      </SettingsRow>

      {error && (
        <p
          id="settings-notification-error"
          data-testid="settings-notifications-error"
          role="alert"
          className="text-xs text-[var(--color-text-secondary)]"
        >
          {error}
        </p>
      )}

      <fieldset className="space-y-3 border-t border-[var(--color-border)] pt-3">
        <legend className="sr-only">Notification channels</legend>
        {NOTIFICATION_CHANNELS.map((channel) => (
          <SettingsRow
            key={channel.id}
            htmlFor={`settings-channel-${channel.id}`}
            title={channel.label}
            description={channel.description}
          >
            <input
              id={`settings-channel-${channel.id}`}
              type="checkbox"
              checked={preferences.channels[channel.id]}
              onChange={(event) =>
                onChange({
                  ...preferences,
                  channels: {
                    ...preferences.channels,
                    [channel.id]: event.target.checked,
                  },
                })
              }
              className="mt-1 h-4 w-4"
            />
          </SettingsRow>
        ))}
      </fieldset>
    </SettingsCard>
  );
}
