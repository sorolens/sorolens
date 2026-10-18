"use client";

import { useLocale } from "@/lib/i18n";

export function LocalePreferences() {
  const { locale, setLocale, timeZone, setTimeZone, t } = useLocale();

  return (
    <div className="flex items-center gap-2">
      <label className="sr-only" htmlFor="locale-selector">
        {t("preferences.language")}
      </label>
      <select
        id="locale-selector"
        aria-label={t("preferences.language")}
        value={locale}
        onChange={(event) => {
          const value = event.target.value;
          if (value === "en" || value === "es") setLocale(value);
        }}
        className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-2 py-1 text-xs text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none"
      >
        <option value="en">English</option>
        <option value="es">Español</option>
      </select>
      <label className="sr-only" htmlFor="timezone-selector">
        {t("preferences.timezone")}
      </label>
      <select
        id="timezone-selector"
        aria-label={t("preferences.timezone")}
        value={timeZone}
        onChange={(event) => {
          const value = event.target.value;
          if (value === "UTC" || value === "local") setTimeZone(value);
        }}
        className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-card)] px-2 py-1 text-xs text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none"
      >
        <option value="local">{t("preferences.local")}</option>
        <option value="UTC">{t("preferences.utc")}</option>
      </select>
    </div>
  );
}
