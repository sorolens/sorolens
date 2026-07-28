# Contributing to Sorolens
Thank you for your interest. This document covers everything you need to make a contribution: local setup, how to claim an issue, branch naming, commit format, PR requirements, and a full walkthrough of the most common first contribution (adding an XDR type decoder).
---
## Table of contents
1. [Prerequisite versions](#prerequisite-versions)
2. [Local setup per package](#local-setup-per-package)
3. [How to claim an issue](#how-to-claim-an-issue)
4. [Branch naming](#branch-naming)
5. [Commit format](#commit-format)
6. [PR checklist](#pr-checklist)
7. [Running tests](#running-tests)
8. [Walkthrough: adding a new XDR type decoder](#walkthrough-adding-a-new-xdr-type-decoder)
---
## Prerequisite versions
| Tool | Required version | Install |
|---|---|---|
| Go | 1.23.x | https://go.dev/dl/ |
| Node | 22.x | https://nodejs.org/ or `nvm install 22` |
| pnpm | 9.x | `npm install -g pnpm@9` |
| Rust | stable (latest) | `rustup install stable` |
| Docker | any recent | https://docs.docker.com/get-docker/ |
| golangci-lint | 1.59.x | https://golangci-lint.run/usage/install/ |
Check your versions:
```bash
go version          # go1.23.x
node --version      # v22.x.x
pnpm --version      # 9.x.x
rustc --version     # rustc 1.xx.x (stable)
golangci-lint --version
```
---
## Local setup per package
### Start shared infrastructure
```bash
docker compose up -d
# Postgres 16 on localhost:5432 (user: sorolens, password: sorolens, db: sorolens)
# Redis on localhost:6379
```
### `apps/api` - Go API
```bash
cd apps/api
cp .env.example .env         # edit DATABASE_URL and REDIS_URL if needed
go run ./cmd/api             # starts on :8080
```
Environment variables (see `.env.example`):
| Variable | Default | Notes |
|---|---|---|
| `DATABASE_URL` | `postgres://sorolens:sorolens@localhost:5432/sorolens` | pgx connection string |
| `REDIS_URL` | `redis://localhost:6379` | |
| `SOROBAN_RPC_URL` | `https://soroban-testnet.stellar.org` | |
| `PORT` | `8080` | |
### `services/indexer` - Go indexer
```bash
cd services/indexer
cp .env.example .env
go run ./cmd/migrate up      # apply schema migrations
go run ./cmd/sorolens index --once   # run one indexer cycle and exit
```
To run continuously (equivalent to the cron in production):
```bash
go run ./cmd/sorolens index --interval 5m
```
### `apps/web` - Next.js dashboard
```bash
cd apps/web
cp .env.local.example .env.local   # set NEXT_PUBLIC_API_URL=http://localhost:8080
pnpm install
pnpm dev     # starts on :3000
```
### `packages/xdr` - TypeScript XDR decoder
```bash
cd packages/xdr
pnpm install
pnpm build   # compiles TypeScript to dist/
pnpm test    # runs vitest
```
### `cli/` - Go CLI
```bash
cd cli
go build -o sorolens ./cmd/sorolens
./sorolens --help
```
### `contracts/counter` - Rust fixture contract
```bash
cd contracts/counter
cargo check
cargo test
# To build the WASM:
cargo build --target wasm32v1-none --release
```
---
## How to claim an issue
1. Find an open issue labeled `good first issue`, `help wanted`, or any other label that matches your interest.
2. Leave a comment: "I'd like to work on this." A maintainer will assign it to you within 48 hours.
3. If no response after 48 hours, go ahead and open a PR referencing the issue.
4. Do not open a PR for an issue assigned to someone else without first checking with them.
If you want to work on something not yet tracked as an issue, open an issue first and describe what you plan to do. This avoids duplicate work.
---
## Branch naming
Format: `<type>/<short-description>`
The `<type>` must be one of the Conventional Commits types listed below.
Examples:
```
feat/xdr-decode-map-type
fix/indexer-duplicate-events-on-restart
docs/add-xdr-decoder-walkthrough
chore/upgrade-golangci-lint
test/invocations-api-pagination
refactor/split-indexer-lock-module
```
Use lowercase and hyphens only. No slashes inside the description segment.
---
## Commit format
Sorolens follows [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/).
Format:
```
<type>(<scope>): <short description>
[optional body]
[optional footer(s)]
```
The short description must:
- Be 72 characters or fewer.
- Use the imperative mood ("add", not "adds" or "added").
- Not end with a period.
### Types and examples
| Type | When to use | Example |
|---|---|---|
| `feat` | A new feature visible to users or consumers of the API/CLI/package | `feat(xdr): add decoder for ScMap type` |
| `fix` | A bug fix | `fix(indexer): prevent duplicate event upsert on cursor retry` |
| `docs` | Documentation only (README, CONTRIBUTING, code comments, etc.) | `docs(contributing): add xdr decoder walkthrough` |
| `test` | Adding or correcting tests, no production code changes | `test(api): add pagination edge case for empty event set` |
| `refactor` | Code change that neither fixes a bug nor adds a feature | `refactor(indexer): extract lock logic into its own package` |
| `chore` | Maintenance: dependency updates, CI config, build scripts, tooling | `chore: upgrade golangci-lint to 1.59` |
| `perf` | A change that improves performance | `perf(api): add composite index on events(contract_id, ledger)` |
| `ci` | Changes to GitHub Actions workflows only | `ci: cache pnpm store in node job` |
| `revert` | Reverts a previous commit | `revert: feat(xdr): add decoder for ScMap type` |
### Breaking changes
Add `BREAKING CHANGE:` in the commit footer, or append `!` after the type:
```
feat(api)!: rename /events cursor field from pagingToken to cursor
BREAKING CHANGE: the cursor field in getEvents responses is now named
`cursor` instead of `pagingToken` to align with the RPC response shape.
```
### Scope values
Use the package or app name as the scope: `api`, `indexer`, `web`, `xdr`, `cli`, `contracts`, `ci`, `docs`.
---
## PR checklist
Before marking your PR ready for review, verify each item:
- [ ] The PR title follows the Conventional Commits format.
- [ ] The PR description includes `Closes #<issue-number>` (or `Refs #<issue-number>` if it partially addresses an issue).
- [ ] All new behavior is covered by tests (unit or integration).
- [ ] Existing tests pass locally (`go test -race ./...` and `pnpm vitest run`).
- [ ] `golangci-lint run` passes with no new warnings for Go changes.
- [ ] TypeScript compiles without errors (`pnpm build` in `packages/xdr` or `apps/web`).
- [ ] No secrets, API keys, or private keys are committed. Check with `git diff --staged`.
- [ ] If you changed the schema, a migration file is included in `services/indexer/migrations/`.
- [ ] If you changed the REST API surface, `ARCHITECTURE.md` is updated.
- [ ] If you changed environment variables, `.env.example` is updated.
---
## Running tests
### Go (API, indexer, CLI)
Run from the module root of each package:
```bash
# API
cd apps/api
go test -race ./...
# Indexer
cd services/indexer
go test -race ./...
# CLI
cd cli
go test -race ./...
```
To run the linter:
```bash
golangci-lint run ./...
```
Integration tests that require a live database are in files ending `_integration_test.go` and are skipped unless `INTEGRATION=true` is set:
```bash
INTEGRATION=true DATABASE_URL=postgres://... go test -race ./...
```
### TypeScript (XDR package and dashboard)
```bash
# XDR package
cd packages/xdr
pnpm vitest run
# Dashboard
cd apps/web
pnpm vitest run        # unit tests
pnpm build             # verifies the Next.js build succeeds
```
### Rust (fixture contract)
```bash
cd contracts/counter
cargo test
cargo check --target wasm32v1-none
```
---
## Walkthrough: adding a new XDR type decoder
This is the most common first contribution. The `packages/xdr` package exposes `decodeScVal(base64: string): unknown`, which converts a base64-encoded XDR `ScVal` into a plain JavaScript value. When the decoder encounters an unrecognized `ScVal` type, it returns the raw base64 string unchanged. Adding a new type means handling that case.
### Step 1: understand the ScVal type you are adding
ScVal is defined in [stellar-xdr](https://github.com/stellar/stellar-xdr). Open the XDR source or the `@stellar/stellar-sdk` type definitions and find the `ScVal` union arm you want to handle. For this walkthrough we will add `scvMap` (a key-value map of `ScVal` to `ScVal`).
```typescript
// In @stellar/stellar-sdk, ScVal arms include:
// .switch().name === "scvMap"
// .value() returns xdr.ScMap, which is an array of xdr.ScMapEntry
// each ScMapEntry has .key() and .val(), both of type xdr.ScVal
```
### Step 2: locate the decoder file
```
packages/xdr/
  src/
    decode.ts       <- the main decoder, add your case here
    decode.test.ts  <- tests, add a test here
  index.ts          <- re-exports decodeScVal and decodeTopics
```
Open `src/decode.ts`. You will find a `switch` statement on `scVal.switch().name`:
```typescript
export function decodeScVal(base64: string): unknown {
  const scVal = xdr.ScVal.fromXDR(base64, "base64");
  switch (scVal.switch().name) {
    case "scvBool":
      return scVal.b();
    case "scvU32":
      return scVal.u32();
    case "scvI32":
      return scVal.i32();
    case "scvU64":
      return BigInt(scVal.u64().toString());
    case "scvI64":
      return BigInt(scVal.i64().toString());
    case "scvString":
      return scVal.str().toString("utf8");
    case "scvSymbol":
      return scVal.sym().toString("ascii");
    case "scvBytes":
      return scVal.bytes().toString("hex");
    case "scvAddress":
      return Address.fromScVal(scVal).toString();
    case "scvVec": {
      const vec = scVal.vec();
      return vec ? vec.map((item) => decodeScVal(item.toXDR("base64"))) : [];
    }
    // ... other cases
    default:
      // unrecognized type: return raw base64 so callers can handle it
      return base64;
  }
}
```
### Step 3: add the `scvMap` case
A map should decode to a plain JavaScript object. Keys that decode to a string or symbol become object keys; keys of other types are converted to their string representation.
```typescript
case "scvMap": {
  const entries = scVal.map();
  if (!entries) return {};
  const result: Record<string, unknown> = {};
  for (const entry of entries) {
    const decodedKey = decodeScVal(entry.key().toXDR("base64"));
    const decodedVal = decodeScVal(entry.val().toXDR("base64"));
    // Use the decoded key as a string property name.
    result[String(decodedKey)] = decodedVal;
  }
  return result;
}
```
### Step 4: write a test
Open `src/decode.test.ts` and add a test. The test builds the XDR from scratch using the SDK so it does not depend on a live network.
```typescript
import { describe, it, expect } from "vitest";
import { xdr } from "@stellar/stellar-sdk";
import { decodeScVal } from "./decode";
describe("decodeScVal", () => {
  // ... existing tests ...
  it("decodes scvMap to a plain object", () => {
    const entry = new xdr.ScMapEntry({
      key: xdr.ScVal.scvSymbol("balance"),
      val: xdr.ScVal.scvU32(42),
    });
    const map = xdr.ScVal.scvMap([entry]);
    const base64 = map.toXDR("base64");
    const result = decodeScVal(base64);
    expect(result).toEqual({ balance: 42 });
  });
  it("returns empty object for an empty scvMap", () => {
    const map = xdr.ScVal.scvMap([]);
    const result = decodeScVal(map.toXDR("base64"));
    expect(result).toEqual({});
  });
});
```
### Step 5: run the tests
```bash
cd packages/xdr
pnpm vitest run
```
All tests should pass. If you see a TypeScript error, check that you imported any SDK types you need at the top of `decode.ts`.
### Step 6: build and verify
```bash
pnpm build
```
This compiles TypeScript to `dist/`. Fix any compilation errors before committing.
### Step 7: commit and open a PR
```bash
git checkout -b feat/xdr-decode-map-type
git add packages/xdr/src/decode.ts packages/xdr/src/decode.test.ts
git commit -m "feat(xdr): add decoder for scvMap type"
git push origin feat/xdr-decode-map-type
```
Open a PR against `main`. Title: `feat(xdr): add decoder for scvMap type`. Body: `Closes #<issue-number>`.
That is the full cycle. Most XDR decoder contributions follow exactly this pattern.
---
## Troubleshooting
If setup fails, the quickest fixes are usually environment variables or services not running.
### `go: command not found` after installing Go
The Go binary is often installed under `/usr/local/go/bin` or `$HOME/go/bin`, which may not be on your `PATH`.
```bash
export PATH=$PATH:/usr/local/go/bin
# or
export PATH=$PATH:$HOME/go/bin
```
Verify the change with:
```bash
go version
```
Windows-specific: add the Go install directory (typically `C:\Program Files\Go\bin`) and your user Go bin directory (for example `C:\Users\<you>\go\bin`) to `PATH`, then restart your terminal.
### Docker daemon is not running
If `docker compose up -d` fails or `docker ps` returns an error, the Docker daemon is usually the issue.
- Start Docker Desktop on macOS or Windows.
- On Linux, start the service and ensure your user can access it:
```bash
sudo systemctl start docker
sudo usermod -aG docker "$USER"
```
Then verify the daemon is responsive:
```bash
docker info
```
### `pnpm: command not found`
Node may be installed, but pnpm is not yet available on your `PATH`.
```bash
npm install -g pnpm@9
pnpm --version
```
If `npm` itself is missing, install Node first and then rerun the command above.
### `psql: command not found` (Windows)
The PostgreSQL client tools are not installed or their `bin` directory is not on `PATH`.
Install PostgreSQL or add the PostgreSQL `bin` folder (for example `C:\Program Files\PostgreSQL\<version>\bin`) to `PATH`, then restart the terminal and verify:
```powershell
psql --version
```
