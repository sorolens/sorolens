## Summary
<!-- One or two sentences describing what this PR does. -->
## Related issue
Closes #
<!-- If this PR partially addresses an issue rather than closing it, use "Refs #" instead. -->
## Changes made
<!-- List the files changed and briefly explain each change. Bullet points are fine. -->
-
-
-
## How to test
<!-- Step-by-step instructions for a reviewer to verify the change works correctly. -->
1.
2.
3.
## Screenshots (UI changes only)
<!-- If you changed the dashboard, add before/after screenshots here. Delete this section if not applicable. -->
| Before | After |
|---|---|
| | |
## Checklist
- [ ] Tests added or updated for every new or changed behavior.
- [ ] `go test -race ./...` passes locally for any Go changes.
- [ ] `pnpm vitest run` passes locally for any TypeScript changes.
- [ ] `golangci-lint run ./...` passes with no new warnings.
- [ ] `pnpm build` passes in `apps/web` for any dashboard changes.
- [ ] No secrets, API keys, or private keys are present in the diff (`git diff main...HEAD | grep -i key`).
- [ ] `.env.example` updated if new environment variables were added.
- [ ] `ARCHITECTURE.md` updated if the REST API surface changed.
- [ ] Migration file added to `services/indexer/migrations/` if the schema changed.
- [ ] CI is passing on this branch.
