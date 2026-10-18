/**
 * Shared formatting helpers for the web app.
 *
 * Soroban contract IDs are long, unreadable strkeys, so the dashboard renders
 * them as a `head...tail` middle ellipsis. Keeping that logic in one place is
 * what makes every truncated contract ID look the same across the contracts,
 * events, watchdog and storage surfaces.
 */

/** Leading characters kept when truncating a Soroban contract ID. */
export const CONTRACT_ID_HEAD_CHARS = 8;

/** Trailing characters kept when truncating a Soroban contract ID. */
export const CONTRACT_ID_TAIL_CHARS = 8;

const ELLIPSIS = "...";

export type FormatLocale = "en" | "es";
export type TimeZonePreference = "UTC" | "local";

/**
 * Collapse the middle of `value`, keeping `head` leading and `tail` trailing
 * characters separated by a single ellipsis.
 *
 * Values that already fit (`value.length <= head + tail + 3`) are returned
 * unchanged, so short aliases and hashes are never mangled.
 *
 * `truncateMiddle("CDLZFC3SY…FW2J", 8, 8)` -> `"CDLZFC3S...HAGQFW2J"`.
 */
export function truncateMiddle(
  value: string,
  head: number = CONTRACT_ID_HEAD_CHARS,
  tail: number = CONTRACT_ID_TAIL_CHARS
): string {
  if (!value) {
    return value;
  }

  if (value.length <= head + tail + ELLIPSIS.length) {
    return value;
  }

  return `${value.slice(0, head)}${ELLIPSIS}${value.slice(-tail)}`;
}

export function formatNumber(
  value: number,
  locale: FormatLocale = "en",
  options?: Intl.NumberFormatOptions
): string {
  return new Intl.NumberFormat(locale, options).format(value);
}

export function formatDateTime(
  value: string | number | Date,
  locale: FormatLocale = "en",
  timeZone: TimeZonePreference = "UTC",
  options: Intl.DateTimeFormatOptions = {
    dateStyle: "medium",
    timeStyle: "short",
  }
): string {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, {
    ...options,
    timeZone: timeZone === "UTC" ? "UTC" : undefined,
  }).format(date);
}
