"use client";

import { SettingsCard, type SaveState } from "./SettingsCard";
import { useTheme } from "@/components/ThemeProvider";

const THEME_OPTIONS = [
  { value: "light", label: "Light" },
  { value: "dark", label: "Dark" },
  { value: "system", label: "System" },
] as const;

/**
 * Theme card. Theme is owned by `ThemeProvider` (it writes the `theme` key and
 * applies `data-theme` on <html>), so selecting an option updates the whole app
 * immediately and the provider persists it — there is no second copy here.
 */
export function ThemeSection({
  status,
  onSave,
}: {
  status: SaveState;
  onSave: () => boolean;
}) {
  const { theme, setTheme } = useTheme();

  return (
    <SettingsCard
      testId="settings-theme"
      title="Theme"
      description="Choose the colour scheme for the dashboard."
      status={status}
      onSave={onSave}
    >
      <fieldset className="space-y-2">
        <legend className="sr-only">Theme</legend>
        {THEME_OPTIONS.map((option) => (
          <div key={option.value} className="flex items-center gap-3">
            <input
              type="radio"
              id={`settings-theme-${option.value}`}
              name="settings-theme"
              value={option.value}
              checked={theme === option.value}
              onChange={() => setTheme(option.value)}
              className="h-4 w-4"
            />
            <label
              htmlFor={`settings-theme-${option.value}`}
              className="text-sm text-[var(--color-text-primary)]"
            >
              {option.label}
            </label>
          </div>
        ))}
      </fieldset>
    </SettingsCard>
  );
}
