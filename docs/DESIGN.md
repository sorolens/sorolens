# Design Tokens

## Colors

| Token | Value | Usage |
|---|---|---|
| `--color-warning` | `#f59e0b` (amber-500) | Storage TTL entries expiring within 7 days |
| `--color-danger` | `#ef4444` (red-500) | Storage TTL entries expiring within 2 days |
| `--color-safe` | `#22c55e` (green-500) | Storage TTL entries with >7 days remaining |
| `--color-bg-card` | `#1e1e2e` | Card backgrounds |
| `--color-bg-page` | `#11111b` | Page background |
| `--color-border` | `#313244` | Border color |
| `--color-text-primary` | `#cdd6f4` | Primary text |
| `--color-text-secondary` | `#a6adc8` | Secondary text |
| `--color-accent` | `#89b4fa` | Accent / interactive elements |

## Storage TTL Thresholds

| Severity | Threshold | Color Token |
|---|---|---|
| Safe | `> 7 days` (>120,960 ledgers) | `--color-safe` |
| Warning | `2-7 days` (34,560-120,960 ledgers) | `--color-warning` |
| Danger | `< 2 days` (<34,560 ledgers) | `--color-danger` |

## Typography

| Usage | Size | Weight |
|---|---|---|
| Stat card value | 2rem (32px) | 700 |
| Section title | 1.25rem (20px) | 600 |
| Body | 0.875rem (14px) | 400 |
| Mono ID | 0.8125rem (13px) | 400 |
