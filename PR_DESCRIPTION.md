# Implement Contracts Dashboard List Page

## Summary
Implements the contracts list dashboard page at `/contracts` with API integration, cursor pagination, real-time search filtering, a Track Contract modal with validation, and a reusable `DataTable` component.

## What changed
- **Contracts List Page (`apps/web/app/(app)/contracts/page.tsx`)**:
  - Fetches tracked contracts via `listContracts` API endpoint with cursor pagination.
  - Supports client-side search filtering by contract ID and alias/label.
  - Built interactive Track Contract modal with 56-character Soroban contract ID validation (`^C[A-Z0-9]{55}$`), inline API error state handling, keyboard shortcuts (Escape key listener), backdrop dismiss, and post-success table refresh.
  - Added loading skeleton states (`TableSkeleton`) and styled empty state views.
- **Reusable Component (`packages/ui/src/DataTable.tsx`)**:
  - Added shared generic `DataTable<T>` component with custom accessors, column sorting with directional indicators (`▲` / `▼` / `↕`), row click navigation, and loading pulse states.
- **Testing & Vitest Config**:
  - Configured `@sorolens/ui` and `@sorolens/xdr` workspace path aliases in `apps/web/vitest.config.ts`.
  - Added comprehensive test suites in `apps/web/app/(app)/contracts/page.test.tsx` (15 tests) and `packages/ui/src/DataTable.test.tsx` (10 tests).

## Testing
- `npx vitest run` in `apps/web` (15/15 passing)
- `npx vitest run --environment jsdom` in `packages/ui` (12/12 passing)
