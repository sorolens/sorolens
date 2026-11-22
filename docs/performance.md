# Performance budgets and bundle analysis (web)

The dashboard has a performance budget enforced in CI (issue #614): every
pull request builds the production bundle and fails the **Web** check when
any route's first-load JavaScript, or the total shipped client JS, exceeds
the budget recorded in [`apps/web/performance-budget.json`](https://github.com/sorolens/sorolens/blob/main/apps/web/performance-budget.json).

## What is measured

- **Per-route first-load JS (gzipped)** — every `.js` file a route needs,
  deduplicated, taken from `.next/app-build-manifest.json` after
  `pnpm build`. This is what a browser downloads to render the route.
- **Total static JS (gzipped)** — everything under `.next/static/chunks`,
  the ceiling on what any session can download.

Sizes are gzipped because that is what crosses the wire.

## Enforcing and updating budgets

CI runs `node scripts/check-bundle-budget.mjs` (in the existing Web job —
no extra PR check). Locally, from `apps/web`:

```sh
pnpm build && node scripts/check-bundle-budget.mjs
```

If the check fails and the growth is intentional, re-baseline against the
current build (+10% headroom) and commit the updated file with your change:

```sh
node scripts/check-bundle-budget.mjs --update-baseline
```

## Bundle analysis

To see what is inside the bundles, build with the analyzer enabled:

```sh
cd apps/web
ANALYZE=true pnpm build
```

This emits client, server and edge reports (HTML treemap + `stats.json`)
under `apps/web/.next/`. The client report is the one to read for the
budget above: open `.next/client.html` and look for the largest leaves.

## Current state

Baseline taken 2026-11-19 on `main` + this PR (Next.js 15.5, React 19):

| Metric | Value |
| --- | --- |
| First Load JS shared by all routes | ~193 KB |
| Largest route (`/(app)/contracts/[id]/page`) | ~340 KB |
| Total static JS | ~669 KB |
| Routes with a budget | all (see `performance-budget.json`) |

Images use `next/image`: the logo is a static import-sized asset and the
API-served SVG badges render `unoptimized` (dynamic SVGs cannot go through
the optimizer), which still gives layout-stable sizing and alt handling.
