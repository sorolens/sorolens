# Pull Request Template

## Description

This PR adds unit tests for issue #765 covering the following components:

### Test Files Added

| Component | Test File | Tests |
|-----------|-----------|-------|
| `ImportContractsCsv` | `apps/web/components/ImportContractsCsv.test.tsx` | 52 tests covering CSV parsing, import modal interactions, duplicate detection, error handling, and edge cases |
| `SnapshotPanel` | `apps/web/components/SnapshotPanel.test.tsx` | 14 tests covering snapshot downloading, ledger validation, event rendering, error messages, and loading states |
| `StoragePanel` | `apps/web/components/StoragePanel.test.tsx` | 12 tests covering rendering, durability filtering, danger/warning/safe sorting, and TTL legend |
| `StarButton` | `apps/web/components/StarButton.test.tsx` | 7 tests covering watchlist toggle, optimistic updates, and pending state handling |
| `settings` | `apps/web/lib/settings.test.ts` | 25 tests covering email validation, settings persistence, round-tripping, corrupt blob recovery, and unavailable storage fallback |

### Canonical Domain Fix

- Fixed domain from `sorolens.dev` → `sorolens.xyz` in `settings.test.ts` per domain validation requirements

### Conventional Commits

All commits follow the conventional commits format:
- `test(web): add unit tests for settings, import, snapshot and storage components (#765)`
- `test(web): fix canonical domain in settings test (sorolens.xyz)`

### Check Status

- All 5 test files created following upstream patterns from `origin/test/web-settings-import-snapshot-storage`
- Canonical domain verified as `sorolens.xyz`
- Branch `test/web-settings-import-snapshot-storage` pushed to fork
- PR title follows conventional commits specification