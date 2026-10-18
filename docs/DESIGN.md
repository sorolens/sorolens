# Design Tokens## Colors

All colors are **semantic tokens** declared in `apps/web/app/globals.css`, with
one block per theme (`[data-theme="light"]` and `[data-theme="dark"]`). Both
blocks define the same keys. Components must never use raw Tailwind palette
colors (`text-red-400` etc.) — they were designed for dark mode only and fail
light-mode contrast.

| Token                                 | Usage                                              |
| ------------------------------------- | -------------------------------------------------- |
| `--color-bg-page`, `--color-bg`       | Page background                                    |
| `--color-bg-card`, `--color-surface`  | Cards, dialogs, elevated surfaces                  |
| `--color-bg-elevated`                 | Inputs, skeleton rows, zebra rows                  |
| `--color-bg-hover`, `--color-bg-active` | Row/button hover and active states               |
| `--color-border`, `--color-border-hover` | Borders                                          |
| `--color-text-primary/secondary/muted` | Text hierarchy                                    |
| `--color-accent`, `--color-info`, `--color-warning`, `--color-danger`, `--color-safe` | Interactive + status colors |
| `--color-*-bg` (accent/info/warning/danger/safe) | Tinted chip and banner backgrounds     |
| `--chart-1` … `--chart-6`, `--color-on-chart` | Categorical chart/flame-graph palette + the text color readable on top of it |

### Contrast audit (issue #609)

`apps/web/lib/design-tokens.test.ts` enforces, in **both** themes:

1. every `var(--…)` used in source is defined (no silently-falling styles);
2. WCAG AA (≥ 4.5:1) for every composed foreground/background pair
   (body/secondary/muted text on page and cards, status text on its tint,
   solid-button labels, chart-fill labels);
3. no raw Tailwind palette classes remain in components.

Visual check: with `pnpm dev`, the theme toggle was exercised on the landing,
contracts list/detail, events, watchdog and rules pages in both themes — chips,
banners (offline, bulk actions), the flame graph and status dots all render
with theme-appropriate tints and AA contrast. `pnpm test:e2e` also runs an axe
scan over the key pages (`tests/e2e/a11y.spec.ts`, issue #611).

## Storage TTL Thresholds

| Severity | Threshold                           | Color Token       |
| -------- | ----------------------------------- | ----------------- |
| Safe     | `> 7 days` (>120,960 ledgers)       | `--color-safe`    |
| Warning  | `2-7 days` (34,560-120,960 ledgers) | `--color-warning` |
| Danger   | `< 2 days` (<34,560 ledgers)        | `--color-danger`  |

## Typography

| Usage           | Size             | Weight |
| --------------- | ---------------- | ------ |
| Stat card value | 2rem (32px)      | 700    |
| Section title   | 1.25rem (20px)   | 600    |
| Body            | 0.875rem (14px)  | 400    |
| Mono ID         | 0.8125rem (13px) | 400    |
