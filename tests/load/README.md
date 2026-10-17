# Load tests (k6)

Issue #551: a k6 load-test suite for the Sorolens read API, a deterministic
seed generator, and a small smoke run in CI.

## What is covered

Three k6 scripts target the highest-volume anonymous read endpoints:

| Script             | Endpoint                                       | k6 op tag               |
| ------------------ | ---------------------------------------------- | ----------------------- |
| `k6/events.js`       | `GET /api/v1/contracts/{id}/events`            | `listContractEvents`      |
| `k6/invocations.js`  | `GET /api/v1/contracts/{id}/invocations`       | `listContractInvocations` |
| `k6/search.js`       | `GET /api/v1/search?q=`                        | `searchContracts`         |

`k6/common.js` holds the shared options, thresholds and the contract discovery
used by the per-contract scripts.

## Thresholds

Defined in `k6/common.js` and enforced by k6 (a breached threshold makes
`k6 run` exit non-zero, which fails the job):

- `http_req_failed: rate < 1%`
- `http_req_duration: p(95) < 1000ms` (smoke-friendly default)

Tune without editing the scripts via env: `P95_MS`, `MAX_FAIL_RATE`, `VUS`,
`DURATION`.

## Seed generator

`apps/api/cmd/seedload` populates Postgres with schema-valid fixtures. It is
deterministic (`-seed`), so runs are reproducible.

```bash
DATABASE_URL=postgres://sorolens:sorolens@localhost:5432/sorolens?sslmode=disable \
  go run ./cmd/seedload \
    -events 1000000 -invocations 250000 -contracts 50 -seed 42 -batch 2000
```

The issue's "~1M events" target is the default. For a quick local run against
the docker-compose stack, apply migrations first, then seed a smaller set.

## Running locally

```bash
# 1. Bring up Postgres + Redis and apply migrations (see docker-compose.yml).
docker compose up -d postgres redis
for f in $(ls apps/api/internal/db/migrations/*.up.sql | sort); do \
  PGPASSWORD=sorolens psql -h 127.0.0.1 -U sorolens -d sorolens -f "$f"; done

# 2. Seed.
(cd apps/api && DATABASE_URL=postgres://sorolens:sorolens@localhost:5432/sorolens?sslmode=disable \
  go run ./cmd/seedload -events 20000 -invocations 5000 -contracts 20)

# 3. Start the API.
(cd apps/api && DATABASE_URL=postgres://sorolens:sorolens@localhost:5432/sorolens?sslmode=disable \
  REDIS_URL=redis://localhost:6379 PORT=8080 go run .)

# 4. Run a script (needs k6 installed).
k6 run tests/load/k6/events.js
```

Optional env for the scripts: `BASE_URL`, `API_KEY`, `CONTRACT_ID`,
`SEARCH_QUERY`, `EVENTS_LIMIT`, `INVOCATIONS_LIMIT`, `SEARCH_LIMIT`.

## CI smoke run

`.github/workflows/load-smoke.yml` stands up Postgres + Redis, applies
migrations, seeds a small dataset, boots the API, and runs all three scripts
with the thresholds above. It runs on a **weekly schedule and manual dispatch**
(not per-PR, since it is heavyweight).

## Status / honesty

The seeder is compiled, vetted, `gofmt`-clean and covered by DB-free unit tests
(`go test ./cmd/seedload`). The k6 scripts pass `node --check` ESM syntax
validation. Neither the k6 scripts nor `load-smoke.yml` were executed against a
live stack in the authoring environment (no k6 binary / Postgres available), so
the end-to-end smoke run has not been observed green here.
