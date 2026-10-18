# SoroLens Web Dashboard (`@sorolens/web`)

The official web dashboard for **SoroLens** — a real-time analytics, observability, and monitoring platform for Soroban smart contracts on the Stellar network.

---

## Table of Contents

- [Overview & Tech Stack](#overview--tech-stack)
- [Prerequisites & Setup](#prerequisites--setup)
- [Environment Variables](#environment-variables)
- [Available Scripts](#available-scripts)
- [Route Map](#route-map)
- [End-to-End (E2E) Mock Strategy](#end-to-end-e2e-mock-strategy)
- [How to Add a Page](#how-to-add-a-page)
- [Project Structure](#project-structure)

---

## Overview & Tech Stack

The dashboard is built with the Next.js App Router and designed for low-latency contract inspection, live event streaming, cross-contract transaction call-tracing, watchdog alert configuration, and offline-capable alerting.

| Layer                       | Technologies                                                                          | Purpose                                                                        |
| :-------------------------- | :------------------------------------------------------------------------------------ | :----------------------------------------------------------------------------- |
| **Framework**               | [Next.js 15](https://nextjs.org/) (App Router), [React 19](https://react.dev/)        | Hybrid static (ISR) and dynamic server-rendered pages                          |
| **Styling**                 | [Tailwind CSS v4](https://tailwindcss.com/), CSS variables                            | Themeable UI tokens (dark, light, system modes)                                |
| **Data Fetching**           | [SWR](https://swr.vercel.app/), native `fetch`, SSE (EventSource)                     | Client-side cache, stale-while-revalidate, live telemetry streaming            |
| **PWA & Offline**           | [@ducanh2912/next-pwa](https://github.com/DuCanhDe/next-pwa), `idb`                   | Service worker caching, Web Push notifications, offline IndexedDB alert queue  |
| **Charts & Visualizations** | [Recharts](https://recharts.org/), `@tanstack/react-virtual`                          | Activity sparklines, invocation trend charts, flame graphs, virtualized tables |
| **UI Components**           | [cmdk](https://cmdk.paco.me/), `@sorolens/ui`                                         | Quick-search command palette, shared design primitives                         |
| **Error Tracking**          | [@sentry/nextjs](https://sentry.io/), Plausible Analytics                             | Client-side error tracking and privacy-preserving analytics                    |
| **Unit Testing**            | [Vitest](https://vitest.dev/), [@testing-library/react](https://testing-library.com/) | JSDOM component tests and isolated state logic                                 |
| **E2E Testing**             | [Playwright](https://playwright.dev/)                                                 | End-to-end integration flows with mocked API responses                         |

---

## Prerequisites & Setup

### Requirements

- **Node.js**: `>=20.0.0`
- **Package Manager**: `pnpm` (version `11.17.0` recommended, matching monorepo config)

### Installation

Install dependencies from the repository root:

```bash
pnpm install
```

### Local Development

1. Navigate to `apps/web`:
   ```bash
   cd apps/web
   ```
2. Copy the sample environment file:
   ```bash
   cp .env.example .env.local
   ```
3. Start the development server:
   ```bash
   pnpm dev
   ```
4. Open [http://localhost:3000](http://localhost:3000) in your browser.

> [!NOTE]
> The app defaults to connecting to a local SoroLens API backend at `http://localhost:8080`. If you do not have the Go backend running, many pages will render fallback/empty states or allow navigation through cached and mock data.

### Production Build

```bash
# Compile TypeScript OpenAPI types and build the production bundle
pnpm build

# Start the compiled production server
pnpm start
```

---

## Environment Variables

Configuration is handled through standard Next.js environment files (`.env.local`, `.env.production`).

Variables prefixed with `NEXT_PUBLIC_` are bundled and exposed to the browser. All other variables are server-only secrets.

To configure your local environment, copy the template:

```bash
cp .env.example .env.local
```

### Variable Reference

| Variable                          | Scope           | Type      | Default                         | Description                                                                                                                       |
| :-------------------------------- | :-------------- | :-------- | :------------------------------ | :-------------------------------------------------------------------------------------------------------------------------------- |
| `NEXT_PUBLIC_API_URL`             | Client & Server | Optional  | `http://localhost:8080`         | Base URL pointing to the SoroLens REST API backend.                                                                               |
| `API_URL`                         | Server-only     | Optional  | `http://localhost:8080`         | Server-side override / fallback for the REST API URL in server components.                                                        |
| `NEXT_PUBLIC_APP_URL`             | Client & Server | Optional  | `http://localhost:3000`         | Canonical public app URL used in `robots.txt` (`https://sorolens.dev` in production).                                             |
| `NEXT_PUBLIC_SITE_URL`            | Client & Server | Optional  | `http://localhost:3000`         | Canonical site origin used in `sitemap.xml` generation.                                                                           |
| `NEXT_PUBLIC_PWA_ENABLED`         | Client & Server | Optional  | `false`                         | Set to `"true"` to enable service worker registration in development mode. Always active in production.                           |
| `VAPID_PUBLIC_KEY`                | Client & Server | Optional* | —                               | Base64url-encoded EC P-256 public key for Web Push. Exposed via `/api/push/key`. Required only if push notifications are enabled. |
| `VAPID_PRIVATE_KEY`               | Server-only     | Secret*   | —                               | Base64url-encoded EC P-256 private key for Web Push. **Never commit or expose publicly.**                                         |
| `VAPID_SUBJECT`                   | Server-only     | Optional  | `mailto:admin@sorolens.dev`     | Contact URI for the VAPID JWT `sub` claim.                                                                                        |
| `PUSH_INTERNAL_SECRET`            | Server-only     | Secret*   | —                               | Shared secret token guarding `PUT /api/push/subscribe` (`x-push-secret` header).                                                  |
| `NEXT_PUBLIC_SENTRY_DSN`          | Client & Server | Optional  | —                               | Client-side Sentry DSN for frontend exception tracking. Leave empty to disable.                                                   |
| `NEXT_PUBLIC_PLAUSIBLE_DOMAIN`    | Client & Server | Optional  | —                               | Domain for Plausible Analytics. Only injected into production builds.                                                             |
| `INDEXER_METRICS_URL`             | Server-only     | Optional  | `http://localhost:9100/metrics` | Prometheus metrics URL of the indexer service scraped by `/api/status/indexer`.                                                   |
| `NEXT_PUBLIC_INDEXER_METRICS_URL` | Client & Server | Optional  | `http://localhost:9100/metrics` | Public fallback alias for `INDEXER_METRICS_URL`.                                                                                  |
| `API_PORT`                        | Server-only     | Optional  | `8080`                          | Target backend port used by Playwright during E2E test runs.                                                                      |

_\* Optional for local development; required for production Web Push features. Generate keys with `npx web-push generate-vapid-keys`._

See [.env.example](.env.example) for detailed comments and safe development placeholders.

---

## Available Scripts

Run these scripts from `apps/web/` (or from the root using `pnpm --filter @sorolens/web <script>`):

| Command                 | Action                                                                             |
| :---------------------- | :--------------------------------------------------------------------------------- |
| `pnpm dev`              | Starts the Next.js development server on port 3000 with Fast Refresh.              |
| `pnpm build`            | Automatically runs `openapi:generate`, then compiles the Next.js production build. |
| `pnpm start`            | Runs the compiled Next.js server on port 3000.                                     |
| `pnpm lint`             | Runs Next.js ESLint rules and code style checks.                                   |
| `pnpm typecheck`        | Runs `tsc --noEmit` to verify TypeScript typings across the app.                   |
| `pnpm test`             | Runs `openapi:check` followed by Vitest in a JSDOM environment.                    |
| `pnpm test:e2e`         | Runs the Playwright end-to-end integration test suite.                             |
| `pnpm openapi:generate` | Parses `docs/openapi.yaml` and regenerates `lib/openapi.generated.ts`.             |
| `pnpm openapi:check`    | Verifies that `lib/openapi.generated.ts` is up-to-date with `docs/openapi.yaml`.   |

---

## Route Map

The application uses Next.js App Router. Routes inside the `(app)` route group share the primary application layout ([`app/(app)/layout.tsx`](<app/(app)/layout.tsx>)) with navigation, breadcrumbs, search, network selection, and offline banners.

### User-Facing Pages

| Path                      | File                                                                                       | Type            | Description                                                                                 |
| :------------------------ | :----------------------------------------------------------------------------------------- | :-------------- | :------------------------------------------------------------------------------------------ |
| `/`                       | [`app/page.tsx`](app/page.tsx)                                                             | Static (ISR 1m) | Landing / marketing page with global contract stats and highlights.                         |
| `/contracts`              | [`app/(app)/contracts/page.tsx`](<app/(app)/contracts/page.tsx>)                           | Dynamic         | Contracts directory with search, filtering by network/status, and pagination.               |
| `/contracts/new`          | [`app/(app)/contracts/new/page.tsx`](<app/(app)/contracts/new/page.tsx>)                   | Static          | Contract tracking wizard to validate and register new Soroban contracts.                    |
| `/contracts/[id]`         | [`app/(app)/contracts/[id]/page.tsx`](<app/(app)/contracts/[id]/page.tsx>)                 | Dynamic         | Contract detail view: event logs, invocations, storage entries, WASM spec, and changelog.   |
| `/compare`                | [`app/(app)/compare/page.tsx`](<app/(app)/compare/page.tsx>)                               | Dynamic         | Side-by-side contract comparison tool for activity, storage, and invocation metrics.        |
| `/events`                 | [`app/(app)/events/page.tsx`](<app/(app)/events/page.tsx>)                                 | Static          | Global contract events feed with filtering by contract ID, topic, and type.                 |
| `/events/live`            | [`app/(app)/events/live/page.tsx`](<app/(app)/events/live/page.tsx>)                       | Static          | Real-time live contract events ticker.                                                      |
| `/live`                   | [`app/(app)/live/page.tsx`](<app/(app)/live/page.tsx>)                                     | Static          | Live telemetry dashboard featuring SSE streaming, hot contracts, and throughput sparklines. |
| `/invocations`            | [`app/(app)/invocations/page.tsx`](<app/(app)/invocations/page.tsx>)                       | Static          | Transaction invocations viewer and cross-contract call-trace flamegraph explorer.           |
| `/groups`                 | [`app/(app)/groups/page.tsx`](<app/(app)/groups/page.tsx>)                                 | Static          | Contract groups directory to organize and monitor sets of related contracts.                |
| `/groups/[id]`            | [`app/(app)/groups/[id]/page.tsx`](<app/(app)/groups/[id]/page.tsx>)                       | Dynamic         | Contract group dashboard with aggregated metrics across group members.                      |
| `/watchdog`               | [`app/(app)/watchdog/page.tsx`](<app/(app)/watchdog/page.tsx>)                             | Static          | Watchdog monitor: uptime percentages, health checks, and active incident alerts.            |
| `/watchdog/[id]`          | [`app/(app)/watchdog/[id]/page.tsx`](<app/(app)/watchdog/[id]/page.tsx>)                   | Dynamic         | Contract watchdog health metrics, historical SLA uptime, and incident details.              |
| `/watchdog/notifications` | [`app/(app)/watchdog/notifications/page.tsx`](<app/(app)/watchdog/notifications/page.tsx>) | Static          | Notification preferences: Web Push, Slack, Discord, and webhook destinations.               |
| `/watchdog/reports`       | [`app/(app)/watchdog/reports/page.tsx`](<app/(app)/watchdog/reports/page.tsx>)             | Static          | Monthly uptime and SLA availability reports with export options.                            |
| `/watchlist`              | [`app/(app)/watchlist/page.tsx`](<app/(app)/watchlist/page.tsx>)                           | Static          | User's bookmarked contracts saved across sessions.                                          |
| `/rules`                  | [`app/(app)/rules/page.tsx`](<app/(app)/rules/page.tsx>)                                   | Static          | Alert rule editor and syntax validator for custom Watchdog alerts.                          |
| `/playground`             | [`app/(app)/playground/page.tsx`](<app/(app)/playground/page.tsx>)                         | Static          | Interactive OpenAPI REST API explorer and request tester.                                   |
| `/settings`               | [`app/(app)/settings/page.tsx`](<app/(app)/settings/page.tsx>)                             | Static          | App settings: network selection (Testnet/Mainnet), API endpoints, API keys, and theme.      |
| `/search`                 | [`app/(app)/search/page.tsx`](<app/(app)/search/page.tsx>)                                 | Dynamic         | Search across contracts, human-readable labels, and transaction traces following network.   |
| `/status`                 | [`app/(app)/status/page.tsx`](<app/(app)/status/page.tsx>)                                 | Static          | System status page displaying indexer sync lag and backend availability.                    |
| `/offline-alerts`         | [`app/offline-alerts/page.tsx`](app/offline-alerts/page.tsx)                               | Static          | Offline alert viewer displaying alerts cached in IndexedDB when disconnected.               |

### API Route Handlers & Metadata

| Path                              | File                                                                                         | Methods                 | Description                                                              |
| :-------------------------------- | :------------------------------------------------------------------------------------------- | :---------------------- | :----------------------------------------------------------------------- |
| `/api/push/subscribe`             | [`app/api/push/subscribe/route.ts`](app/api/push/subscribe/route.ts)                         | `POST`, `DELETE`, `PUT` | Manages Web Push subscriptions and internal push alert dispatches.       |
| `/api/push/vapid-public-key`      | [`app/api/push/vapid-public-key/route.ts`](app/api/push/vapid-public-key/route.ts)           | `GET`                   | Returns the server's public VAPID key for client push registration.      |
| `/api/status/indexer`             | [`app/api/status/indexer/route.ts`](app/api/status/indexer/route.ts)                         | `GET`                   | Proxies and formats Prometheus metrics from the indexer service.         |
| `/api/watchdog/test-notification` | [`app/api/watchdog/test-notification/route.ts`](app/api/watchdog/test-notification/route.ts) | `POST`                  | Dispatches a mock notification to verify alert channel configuration.    |
| `/robots.txt`                     | [`app/robots.ts`](app/robots.ts)                                                             | `GET`                   | Generates crawler directives and sitemap URL from `NEXT_PUBLIC_APP_URL`. |
| `/sitemap.xml`                    | [`app/sitemap.ts`](app/sitemap.ts)                                                           | `GET`                   | Generates search engine sitemap from `NEXT_PUBLIC_SITE_URL`.             |

---

## End-to-End (E2E) Mock Strategy

End-to-end tests are implemented using [Playwright](https://playwright.dev/) under [`tests/e2e/`](tests/e2e/).

### Strategy & Architecture

- **No live backend needed**: Tests run completely independently of Docker, Postgres, or the Go API server. All network requests matching `**/api/v1/**` are intercepted client-side via Playwright's `page.route()` mechanism.
- **Fixture Centralization**: Standard mock responses and test data fixtures are defined in [`tests/e2e/helpers.ts`](tests/e2e/helpers.ts):
  - `mockApi(page, overrides)`: Sets up handlers for contract lists, contract details, event streams, invocations, storage entries, call-traces, health scores, and auth sessions.
  - `blockApi(page)`: Aborts API requests with `connectionrefused` to test offline UI states, retry buttons, and disconnected banners.
  - Realistic fixtures: `CONTRACT_ID`, `TRACE_TX_HASH`, `contractSummary()`, `traceResponse()`, `monitoredContract()`.
- **Pre-warming**: [`playwright.global-setup.ts`](playwright.global-setup.ts) runs before test execution to pre-warm Next.js dev server routes, avoiding per-test compilation timeouts.

### Running E2E Tests Locally

```bash
# Run all end-to-end specs in headless Chromium
pnpm test:e2e

# Run with interactive Playwright UI mode
pnpm exec playwright test --ui

# Run a specific test spec
pnpm exec playwright test tests/e2e/contracts.spec.ts

# Run in debug mode (step-through execution)
pnpm exec playwright test --debug
```

---

## How to Add a Page

Follow these conventions when adding a new page to the dashboard:

### 1. File Location & Route

Add your page component inside `apps/web/app/(app)/<your-route>/page.tsx`.

```tsx
// apps/web/app/(app)/analytics/page.tsx
export default function AnalyticsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Analytics</h1>
      {/* Page content */}
    </div>
  );
}
```

### 2. Layout & Shell Integration

Placing your page under `app/(app)/` ensures it automatically inherits:

- Responsive navigation header and footer
- Breadcrumb navigation (`Breadcrumbs`)
- Network switcher (`NetworkSelector`)
- Theme toggle (`ThemeToggle`)
- Offline alert detection (`useOfflineAlertQueue`)
- SWR cache provider configuration

### 3. Navigation Link (Optional)

If your route should appear in the main navigation:

1. Add a link in [`app/(app)/layout.tsx`](<app/(app)/layout.tsx>):
   ```tsx
   <Link href="/analytics" className="...">
     {t("nav.analytics")}
   </Link>
   ```
2. Add the translation string in [`lib/i18n.tsx`](lib/i18n.tsx):
   ```tsx
   "nav.analytics": "Analytics",
   ```

### 4. Data Fetching Pattern

- Use SWR for client-side fetching with typed helper functions from [`lib/api.ts`](lib/api.ts):
  ```tsx
  "use client";

  import useSWR from "swr";
  import { fetchContracts } from "@/lib/api";
  import { ErrorState } from "@/components/ErrorState";
  import { Skeleton } from "@/components/Skeleton";

  export default function MyPage() {
    const { data, error, isLoading, mutate } = useSWR(
      "/contracts",
      fetchContracts
    );

    if (isLoading) return <Skeleton className="h-64" />;
    if (error)
      return <ErrorState message="Failed to load" onRetry={() => mutate()} />;

    return <div>{/* render data */}</div>;
  }
  ```
- Use [`lib/serverData.ts`](lib/serverData.ts) for server-rendered components where pre-fetching or SEO metadata is needed.

### 5. Expected Testing

- **Component tests**: Create `page.test.tsx` alongside your page using Vitest and `@testing-library/react`.
- **E2E tests**: Add a spec in `tests/e2e/<feature>.spec.ts` using `mockApi()` from `tests/e2e/helpers.ts`.

---

## Project Structure

```text
apps/web/
├── app/                  # Next.js App Router (pages, layouts, API route handlers)
│   ├── (app)/            # Authenticated/dashboard shell routes sharing AppLayout
│   │   ├── compare/      # Multi-contract comparison tool
│   │   ├── contracts/    # Contract directory, detail views, and registration wizard
│   │   ├── events/       # Contract event feeds and live ticker
│   │   ├── groups/       # Contract group management and group dashboards
│   │   ├── invocations/  # Invocations and cross-contract call-trace explorer
│   │   ├── live/         # Real-time SSE telemetry dashboard
│   │   ├── playground/   # OpenAPI REST API testing playground
│   │   ├── rules/        # Watchdog alert rules editor
│   │   ├── search/       # Global search across contracts, labels, and traces
│   │   ├── settings/     # App configuration, API keys, network selector
│   │   ├── status/       # Indexer status and lag indicators
│   │   ├── watchdog/     # Watchdog contract monitor, SLA reports, notification config
│   │   └── watchlist/    # Saved contracts watchlist
│   ├── api/              # Route handlers (/api/push, /api/status, /api/watchdog)
│   ├── layout.tsx        # Root HTML layout (fonts, ThemeProvider, Plausible)
│   ├── page.tsx          # Public landing / marketing page
│   ├── robots.ts         # Dynamic robots.txt metadata route
│   └── sitemap.ts        # Dynamic sitemap.xml metadata route
├── components/           # Reusable UI components (charts, sparklines, tables, modal dialogs)
├── hooks/                # Custom React hooks (useContracts, useEventStream, useOfflineAlertQueue)
├── lib/                  # Utilities, SWR configs, i18n dictionary, API client, VAPID helpers
├── public/               # Static assets (favicons, PWA icons, manifest, logos)
├── scripts/              # Build-time scripts (generate-openapi.mjs)
├── tests/
│   └── e2e/              # Playwright E2E test specs and shared mock fixtures (helpers.ts)
├── worker/               # Custom service worker logic (index.ts) injected into next-pwa
├── .env.example          # Template environment variable file
├── next.config.ts        # Next.js and next-pwa configuration
├── package.json          # Dependencies and package scripts
├── playwright.config.ts  # Playwright test runner configuration
├── tsconfig.json         # TypeScript compiler configuration
└── vitest.config.ts      # Vitest unit test configuration
```
