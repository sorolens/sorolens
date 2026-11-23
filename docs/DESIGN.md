# Design Tokens

All dashboard colors are semantic CSS variables defined in
`apps/web/app/globals.css` for both themes (issue #609). Components must use
`var(--color-...)` tokens instead of raw Tailwind palette classes. The guard in
`apps/web/app/globals.test.ts` fails when a referenced token is undefined in
either theme, when a raw palette class returns, or when a themed text/surface
pair drops below WCAG AA.

## Token families

| Family  | Tokens                                                                                                                                                                                                    | Usage                                                         |
| ------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| Surface | `--color-bg-page`, `--color-bg-card`, `--color-surface`, `--color-bg`, `--color-bg-hover`, `--color-overlay`                                                                                              | Page, card, panel, inset, hover and modal-backdrop fills      |
| Border  | `--color-border`, `--color-border-hover`                                                                                                                                                                  | Default and interactive borders                               |
| Text    | `--color-text-primary`, `--color-text-secondary`, `--color-text-muted`, `--color-text-inverse`                                                                                                            | Body, secondary, muted and inverted text                      |
| Accent  | `--color-accent`, `--color-accent-hover`, `--color-accent-bg`, `--color-accent-border`, `--color-accent-text`, `--color-accent-foreground`                                                                | Links, primary buttons and selected states                    |
| Danger  | `--color-danger`, `--color-danger-hover`, `--color-danger-bg`, `--color-danger-border`, `--color-danger-text`, `--color-danger-foreground`                                                                | Failed calls, destructive actions, error banners              |
| Warning | `--color-warning`, `--color-warning-hover`, `--color-warning-bg`, `--color-warning-border`, `--color-warning-text`, `--color-warning-foreground`                                                          | Pending status, TTL warnings, offline state                   |
| Safe    | `--color-safe`, `--color-safe-hover`, `--color-safe-bg`, `--color-safe-border`, `--color-safe-text`, `--color-safe-foreground`                                                                            | Healthy status, successful calls, verified contracts          |
| Info    | `--color-info`, `--color-info-hover`, `--color-info-bg`, `--color-info-border`, `--color-info-text`, `--color-info-foreground`                                                                            | Backfilling status, informational alerts                      |
| Chart   | `--color-chart-1` … `--color-chart-7`, `--color-chart-grid`, `--color-chart-axis`, `--color-chart-tooltip-bg`, `--color-chart-tooltip-border`, `--color-chart-legend`, `--color-chart-foreground`         | Series, grids, axes, tooltips, legends and flame-graph frames |
| Syntax  | `--color-syntax-string`, `--color-syntax-number`, `--color-syntax-boolean`, `--color-syntax-keyword`, `--color-syntax-function`, `--color-syntax-type`, `--color-syntax-error`, `--color-syntax-property` | Rule-language and JSON highlighting                           |

## Theme values

| Token                    | Light     | Dark      |
| ------------------------ | --------- | --------- |
| `--color-bg-page`        | `#f8fafc` | `#11111b` |
| `--color-bg-card`        | `#ffffff` | `#1e1e2e` |
| `--color-surface`        | `#ffffff` | `#1e1e2e` |
| `--color-bg`             | `#f1f5f9` | `#181825` |
| `--color-bg-hover`       | `#e2e8f0` | `#313244` |
| `--color-border`         | `#e2e8f0` | `#313244` |
| `--color-border-hover`   | `#94a3b8` | `#585b70` |
| `--color-text-primary`   | `#0f172a` | `#cdd6f4` |
| `--color-text-secondary` | `#475569` | `#a6adc8` |
| `--color-text-muted`     | `#64748b` | `#9399b2` |
| `--color-accent`         | `#2563eb` | `#89b4fa` |
| `--color-danger`         | `#b91c1c` | `#f87171` |
| `--color-warning`        | `#b45309` | `#fbbf24` |
| `--color-safe`           | `#15803d` | `#4ade80` |
| `--color-info`           | `#1d4ed8` | `#60a5fa` |

## WCAG AA audit

Contrast is computed by `apps/web/app/globals.test.ts` from the tokens in
`globals.css`. The guard requires at least 4.5:1 for all themed text pairs,
including text on status backgrounds, button foregrounds, and flame-graph text
on every chart series. Automated result at the time of this change:

```text
✓ design tokens > defines every referenced token in both themes
✓ design tokens > has no raw Tailwind palette colors left in dashboard source
✓ design tokens > keeps themed foreground/background pairs at WCAG AA
```

Representative ratios:

| Pair                              | Light   | Dark    |
| --------------------------------- | ------- | ------- |
| primary text / page               | 17.06:1 | 12.97:1 |
| secondary text / card             | 7.58:1  | 7.37:1  |
| muted text / card                 | 4.76:1  | 5.81:1  |
| accent / page                     | 4.94:1  | 8.91:1  |
| danger text / danger background   | 7.60:1  | 7.98:1  |
| warning text / warning background | 6.84:1  | 9.58:1  |
| safe text / safe background       | 6.81:1  | 10.16:1 |
| info text / info background       | 8.01:1  | 8.04:1  |

`viewport.themeColor` now advertises `#f8fafc` for light and `#11111b` for
dark, matching the page tokens instead of the previous unrelated violet.

## Visual check

Run the following for any theme or chart change:

1. `cd apps/web && pnpm dev`.
2. Open `/`, `/contracts`, a contract detail page, `/events/live`, `/watchdog`,
   `/status`, `/docs` and `/about`.
3. Toggle light/dark in the header (and separately with the OS preference) and
   confirm: text is readable on every surface, status badges keep their text
   readable on their tinted backgrounds, destructive buttons are readable, and
   chart series, axes, grid lines, tooltips and flame-graph frames are visible.
4. Open DevTools → Elements and confirm no `var(--color-...)` reference is
   unresolved (unresolved variables show no computed value).
5. `pnpm exec vitest run app/globals.test.ts` must pass.

The automated audit and source scan were run in this change. A browser
screenshot was not captured in this headless environment; step 3 remains the
manual sign-off for a reviewer with a display.

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
