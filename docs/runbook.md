# Operations Runbook

This runbook provides step-by-step incident response and remediation procedures for SoroLens operators. It covers the four primary operational scenarios:

1. [Indexer Lag](#1-indexer-lag)
2. [Dead-Letter Queue (DLQ) Drain](#2-dead-letter-queue-dlq-drain)
3. [Backup and Disaster Recovery (Restore)](#3-backup-and-disaster-recovery-restore)
4. [Key and Secret Rotation](#4-key-and-secret-rotation)

---

## Triage Matrix

Use this matrix to identify which incident procedure applies to your current alert, metric, or log output:

| Symptom / Signal | Suspected Scenario | Runbook Section | Default Severity |
| :--- | :--- | :--- | :--- |
| Alert `SorolensIndexerLagging` firing, `sorolens_indexer_lag_ledgers > 1000`, dashboard contract events delayed | Indexer falling behind chain head (RPC throttle, pass timeout, DB saturation) | [1. Indexer Lag](#1-indexer-lag) | **P1 / High** |
| Log `"event parked in DLQ"`, `GET /api/v1/dlq` returns items, `failed_events` table non-empty | Contract events failing persistent storage after retry exhaustion | [2. DLQ Drain](#2-dead-letter-queue-dlq-drain) | **P2 / Medium** |
| Database corruption, accidental table drop/truncation, host failure, corrupt migration | Loss of primary Postgres state or data integrity compromise | [3. Backup Restore](#3-backup-and-disaster-recovery-restore) | **P0 / Critical** |
| Leaked API key token (`sl_...`), leaked webhook signing secret, compromised server secret, or compliance rotation schedule | Secret exposure or scheduled key lifecycle event | [4. Key Rotation](#4-key-and-secret-rotation) | **P1 / High** (if leak)<br>**P3 / Low** (if scheduled) |

---

## 1. Indexer Lag

### Overview

The indexer (`services/indexer`) polls Soroban RPC nodes for contract events and transactions on configured Stellar networks (`testnet`, `mainnet`, `futurenet`), persisting them into PostgreSQL (`events`, `invocations`, `contract_storage`, `indexer_cursors`, `sync_state`). Indexer lag measures the ledger distance between the latest ledger closed on the network and the last ledger committed by SoroLens for that network:

$$\text{lag} = \text{sorolens\_indexer\_head\_ledger} - \text{sorolens\_indexer\_last\_indexed\_ledger}$$

On Stellar, ledgers close approximately every 5 seconds (~720 ledgers/hour).

---

### Symptoms

- **Alerts**:
  - `SorolensIndexerLagging`: `sorolens_indexer_lag_ledgers > 1000` for `15m` (defined in [`docs/metrics.md`](metrics.md)).
- **Prometheus Metrics** (scraped from `:9100/metrics` or `INDEXER_METRICS_ADDR`):
  - `sorolens_indexer_lag_ledgers{network="..."}` steadily increasing.
  - `sorolens_indexer_last_indexed_ledger{network="..."}` stopped advancing.
  - `sorolens_indexer_run_duration_seconds` bumping against `INDEXER_MAX_DURATION` (default 270s).
- **Log Lines** (`services/indexer` stdout):
  - `"indexer: RPC configuration" err="get latest ledger: ..."`
  - `"soroban rpc error"` or `"request failed with status 429 Too Many Requests"`
  - `"database unavailable" err="failed to connect to ..."`
  - `"max duration exceeded"` or `"context deadline exceeded"`
- **API & UI Behavior**:
  - `GET /contracts/:id` shows stale `last_indexed_ledger`.
  - The Status dashboard (`/status`) reports high ledger lag on the affected network.

---

### Checks and Diagnosis

#### 1. Inspect live Prometheus metrics
Check the current lag and cursor values from the indexer metrics server:

```bash
curl -s http://localhost:9100/metrics | grep -E "sorolens_indexer_(lag_ledgers|head_ledger|last_indexed_ledger)"
```

#### 2. Check the committed database cursor
Verify the cursor recorded in Postgres:

```bash
docker compose exec postgres psql -U sorolens -d sorolens -c \
  "SELECT network, ledger, updated_at, NOW() - updated_at AS age FROM indexer_cursors;"
```

#### 3. Test Soroban RPC node availability and latency
Check whether the configured RPC endpoint is responsive and query the node's current ledger:

```bash
# Replace URL with SOROBAN_RPC_URL (or SOROBAN_RPC_URL_TESTNET / MAINNET)
curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"getLatestLedger"}' \
  "${SOROBAN_RPC_URL:-https://soroban-testnet.stellar.org}" | jq .
```

If HTTP 429 is returned, the public RPC node has throttled your IP. If the response times out or returns HTTP 5xx, the upstream node is degraded.

#### 4. Check Postgres connection pool and locks
Check if the API/indexer is exhausting database connections:

```bash
curl -s http://localhost:8080/metrics | grep sorolens_db_pool
```
Check for long-running blocking queries in Postgres:

```bash
docker compose exec postgres psql -U sorolens -d sorolens -c "
SELECT pid, now() - query_start AS duration, query, state
FROM pg_stat_activity
WHERE state != 'idle' AND pid <> pg_backend_pid()
ORDER BY duration DESC LIMIT 5;"
```

#### 5. Check indexer container logs
Tail the indexer service logs for error traces:

```bash
docker compose logs --tail=100 -f indexer
```

---

### Remediation Commands

#### Option A: Manual one-shot catchup run with debug logging
If the scheduled cron or background worker stalled, run an isolated pass with increased duration budget:

```bash
cd services/indexer
LOG_LEVEL=debug go run . -mode once -max-duration 600s
```

#### Option B: Tune worker concurrency and ledger window
If the indexer cannot keep up with high contract activity, tune the worker concurrency via `INDEXER_WORKERS` (defaults to `GOMAXPROCS`) and adjust `-ledger-window` (default 120960):

```bash
# Run with 8 concurrent contract workers
INDEXER_WORKERS=8 go run ./services/indexer -mode once -ledger-window 60000
```

#### Option C: Failover to an alternate Soroban RPC endpoint
When the public SDF RPC endpoint is degraded or throttled, switch to a commercial provider endpoint (e.g. QuickNode, NowNodes, Ankr) by updating the network endpoint in `.env`:

```bash
# Update .env
SOROBAN_RPC_URL_TESTNET="https://your-fallback-provider.example.com"
# Or for default network:
SOROBAN_RPC_URL="https://your-fallback-provider.example.com"
```
Then restart the continuous indexer container:

```bash
docker compose restart indexer
```

#### Option D: Sharded topology (Single-Node Memory Mode)
For large contract sets on a single machine, run dedicated coordinator and worker roles:

```bash
# Terminal 1: Coordinator
go run ./services/indexer --role coordinator --worker-id coord-1 --shard-store memory

# Terminal 2: Worker 1
go run ./services/indexer --role worker --worker-id worker-1 --shard-store memory
```
*(Note: As documented in [`docs/indexer-sharding.md`](indexer-sharding.md), multi-replica sharding across separate hosts requires a shared Postgres store adapter; `--shard-store memory` is only valid within a single node).*

---

### Verification

1. Confirm `sorolens_indexer_lag_ledgers` drops below 50 ledgers and trends towards 0:
   ```bash
   curl -s http://localhost:9100/metrics | grep sorolens_indexer_lag_ledgers
   ```
2. Confirm Postgres cursor advances:
   ```bash
   docker compose exec postgres psql -U sorolens -d sorolens -c \
     "SELECT network, ledger, updated_at FROM indexer_cursors;"
   ```
3. Inspect recent contract events using the CLI:
   ```bash
   sorolens events <CONTRACT_ID> --limit 5
   ```

---

### Escalation and Rollback Notes

- **RPC Retention Window**: Public Soroban RPC nodes retain events for ~24 hours and transaction history for ~7 days. If indexer lag exceeds the RPC retention horizon, missing historical events cannot be backfilled directly from public RPC. You must point `SOROBAN_RPC_URL` at an archival/history node or restore past data from cold storage / database backup.
- **Stuck Cursor Reset**: If a contract is stuck on a corrupt ledger sequence, verify whether events have been parked in DLQ (see Section 2). Re-indexing a ledger range is safe: `pgstore` batch inserts use `ON CONFLICT DO NOTHING`.

---

## 2. Dead-Letter Queue (DLQ) Drain

### Overview

When the indexer cannot persist a contract event after retries (`maxEventRetries = 3` in `services/indexer/internal/poller/poller.go`), it parks the event in the `failed_events` table rather than failing the entire contract pass (issue #202).

The API exposes endpoints to inspect and requeue parked events:
- `GET /api/v1/dlq`: List dead-lettered events (supports `?limit=` [1–200] and `?cursor=`).
- `POST /api/v1/dlq/{id}/requeue`: Re-attempts batch insertion of the parked event into the `events` table and deletes the row from `failed_events` on success.

> **Tooling Note**: There is **no built-in CLI command** for bulk DLQ draining. Operators must use the REST API (curl / script) or SQL procedures documented below.

---

### Symptoms

- **Log Lines**:
  - `services/indexer` logs:
    ```
    level=WARN msg="event parked in DLQ" event_id=... contract_id=... attempts=3 err="insert retries exhausted"
    ```
- **API Responses**:
  - `GET /api/v1/dlq` returns a non-empty `items` array.
- **Database Status**:
  - `SELECT COUNT(*) FROM failed_events;` returns `> 0`.

---

### Checks and Diagnosis

#### 1. Count parked events and inspect error patterns
Run a query directly against Postgres to categorize failures:

```bash
docker compose exec postgres psql -U sorolens -d sorolens -c "
SELECT error_message, count(*), min(created_at) AS oldest, max(created_at) AS newest
FROM failed_events
GROUP BY error_message
ORDER BY count(*) DESC;"
```

#### 2. Query recent DLQ items via the API
Fetch the latest failed events with payload:

```bash
curl -s -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v1/dlq?limit=5" | jq .
```

#### 3. Inspect a specific failed event payload
Extract the JSON payload for a particular failed event ID:

```bash
docker compose exec postgres psql -U sorolens -d sorolens -c \
  "SELECT id, event_id, contract_id, network, error_message, event_payload FROM failed_events WHERE id = <ID>;"
```

#### 4. Determine root cause
Common root causes include:
- **Temporary Database Lock / Timeout**: Event can be safely re-inserted immediately.
- **Schema / Type Constraint Failure**: A code or migration update is required before the event can be inserted.
- **Duplicate Event Collision**: The event was already inserted under a different attempt.

---

### Remediation Commands

#### Option A: Requeue a single DLQ item via API
Requeue an individual item by its surrogate ID:

```bash
curl -s -X POST -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v1/dlq/<ID>/requeue" | jq .
```
Expected response:
```json
{
  "id": 123,
  "event_id": "000012345-0001",
  "status": "requeued"
}
```

#### Option B: Bulk drain script (Bash + curl + jq)
Drain up to 100 failed events sequentially using the API:

```bash
API_URL="http://localhost:8080"
API_KEY="sl_your_api_key_here"

# Fetch IDs and requeue each
IDS=$(curl -s -H "Authorization: Bearer $API_KEY" "$API_URL/api/v1/dlq?limit=100" | jq -r '.items[].id // empty')

if [ -z "$IDS" ]; then
  echo "DLQ is already empty."
else
  for id in $IDS; do
    echo "Requeueing DLQ item ID: $id..."
    RES=$(curl -s -X POST -H "Authorization: Bearer $API_KEY" "$API_URL/api/v1/dlq/$id/requeue")
    echo "  Result: $RES"
  done
fi
```

#### Option C: Manual database purge (Non-Recoverable or Corrupt Payloads)
If a parked event is malformed and cannot be re-inserted, export it for offline analysis and delete it from `failed_events`:

```bash
# 1. Export payload for post-mortem analysis
docker compose exec postgres psql -U sorolens -d sorolens -c \
  "COPY (SELECT * FROM failed_events WHERE id = <ID>) TO STDOUT WITH CSV HEADER;" > corrupt_event_<ID>.csv

# 2. Delete the unrecoverable item
docker compose exec postgres psql -U sorolens -d sorolens -c \
  "DELETE FROM failed_events WHERE id = <ID>;"
```

---

### Verification

1. Verify `GET /api/v1/dlq` returns no pending items:
   ```bash
   curl -s -H "Authorization: Bearer $API_KEY" "http://localhost:8080/api/v1/dlq" | jq .
   ```
   Output should be: `{"items":[],"next_cursor":""}`.
2. Verify row count in Postgres is zero:
   ```bash
   docker compose exec postgres psql -U sorolens -d sorolens -c \
     "SELECT count(*) FROM failed_events;"
   ```
3. Confirm requeued event exists in `events` table:
   ```bash
   docker compose exec postgres psql -U sorolens -d sorolens -c \
     "SELECT id, contract_id, ledger, topic FROM events WHERE id = '<EVENT_ID>';"
   ```

---

### Escalation and Rollback Notes

- If `POST /api/v1/dlq/{id}/requeue` returns HTTP 500 (`"failed to re-insert event"`), do not delete the row from `failed_events`. The payload remains parked safely until application or schema fixes are applied.
- The `failed_events` table has a `UNIQUE(event_id)` constraint; re-attempting an insert will not corrupt or duplicate the queue.

---

## 3. Backup and Disaster Recovery (Restore)

### Overview

SoroLens utilizes PostgreSQL 16 with WAL (Write-Ahead Logging) archiving enabled. The repository provides two operational scripts:
- [`scripts/backup.sh`](../scripts/backup.sh): Creates a logical backup (`pg_dump`) and physical base backup (`pg_basebackup`) and uploads both to S3 (`$S3_BUCKET`).
- [`scripts/restore.sh`](../scripts/restore.sh): Downloads a base backup from S3, extracts it into `./postgres_data_restore`, and configures `postgresql.auto.conf` and `recovery.signal` for Point-In-Time-Recovery (PITR).

Environment variables:
- `S3_BUCKET`: Target S3 bucket (default: `s3://sorolens-backups`).
- `DB_CONTAINER`: Docker container name (default: `sorolens-postgres-1` or `postgres`).
- `DB_USER`: Postgres user (default: `sorolens`).
- `DB_NAME`: Database name (default: `sorolens`).

---

### Symptoms

- Critical database failure: data volume lost or corrupted.
- Accidental table truncation or erroneous schema modification.
- Disaster recovery failover to a new host or AWS region.

---

### Checks and Diagnosis

#### 1. Confirm target restore timestamp (PITR)
Identify the exact UTC target timestamp right before the corruption or data loss occurred (e.g. `'2026-10-17 14:30:00'`).

#### 2. Verify available base backups and WAL segments in S3
Check the available base backups and WAL archives in S3:

```bash
# List available base backups
aws s3 ls s3://sorolens-backups/base/

# List logical pg_dump backups
aws s3 ls s3://sorolens-backups/logical/

# Check that WAL archive contains segments
aws s3 ls s3://sorolens-backups/wal/ | tail -n 10
```

Select a base backup whose timestamp is **immediately prior** to your target restore time.

---

### Remediation Commands

#### Procedure A: Point-In-Time-Recovery (PITR)

##### Step 1: Stop write traffic to the database
Stop the API and indexer to prevent further writes:

```bash
docker compose stop api indexer
```

##### Step 2: Run `scripts/restore.sh`
Run the restore script with the base backup S3 URI and target UTC time:

```bash
./scripts/restore.sh s3://sorolens-backups/base/base_20261015_120000.tar.gz '2026-10-15 13:45:00'
```

The script performs the following actions:
1. Downloads `base_restore.tar.gz`.
2. Extracts files into `./postgres_data_restore`.
3. Creates `./postgres_data_restore/recovery.signal`.
4. Appends recovery parameters to `./postgres_data_restore/postgresql.auto.conf`:
   ```ini
   restore_command = 'aws s3 cp s3://sorolens-backups/wal/%f %p'
   recovery_target_time = '2026-10-15 13:45:00'
   recovery_target_action = 'promote'
   ```

##### Step 3: Stop the running Postgres container
```bash
docker compose stop postgres
```

##### Step 4: Swap data directories
Mount or copy the restored directory into your Postgres data volume:

```bash
# Backup old data directory as safety measure
docker run --rm -v sorolens_postgres_data:/data -v "$(pwd)":/backup alpine \
  tar -czf /backup/pre_restore_backup.tar.gz -C /data .

# Replace volume contents with restored data
docker run --rm -v sorolens_postgres_data:/data -v "$(pwd)/postgres_data_restore":/restore alpine \
  sh -c "rm -rf /data/* && cp -a /restore/* /data/"
```

##### Step 5: Start Postgres and follow recovery replay
Start the database service:

```bash
docker compose up -d postgres
```
Follow WAL replay in logs until promotion:

```bash
docker compose logs -f postgres
```
Look for log confirmation:
```
starting point-in-time recovery to 2026-10-15 13:45:00+00
restored log file "..." from archive
recovery stopping at 2026-10-15 13:45:00+00
redo done at ...
database system is ready to accept connections
```

---

#### Procedure B: Logical Database Restore (`pg_dump` SQL)

If PITR is not required and you are restoring from a scheduled daily snapshot:

##### Step 1: Download the dump file from S3
```bash
aws s3 cp s3://sorolens-backups/logical/backup_20261015_000000.sql.gz ./backup.sql.gz
```

##### Step 2: Terminate active connections and recreate target DB
```bash
docker compose exec postgres psql -U sorolens -d postgres -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'sorolens' AND pid <> pg_backend_pid();"

docker compose exec postgres psql -U sorolens -d postgres -c "DROP DATABASE IF EXISTS sorolens;"
docker compose exec postgres psql -U sorolens -d postgres -c "CREATE DATABASE sorolens OWNER sorolens;"
```

##### Step 3: Stream the SQL dump into Postgres
```bash
gunzip -c ./backup.sql.gz | docker compose exec -T postgres psql -U sorolens -d sorolens
```

##### Step 4: Run migration check
Verify that the database schema is complete:

```bash
make migrate-check
```

---

### Verification

1. Verify Postgres readiness probe:
   ```bash
   docker compose exec postgres pg_isready -U sorolens -d sorolens
   ```
2. Verify table record counts:
   ```bash
   docker compose exec postgres psql -U sorolens -d sorolens -c "
   SELECT 'contracts' AS table_name, count(*) FROM contracts
   UNION ALL
   SELECT 'events', count(*) FROM events
   UNION ALL
   SELECT 'invocations', count(*) FROM invocations
   UNION ALL
   SELECT 'contract_storage', count(*) FROM contract_storage;"
   ```
3. Test API readiness endpoint:
   ```bash
   curl -s http://localhost:8080/readyz | jq .
   ```
   Should return: `{"status":"ok"}`.
4. Restart application workers:
   ```bash
   docker compose up -d api indexer
   ```

---

### Escalation and Rollback Notes

- Never discard the pre-restore data volume until all core table queries and the API health checks pass.
- If Postgres stops during recovery with an archive failure, verify IAM access to `s3://sorolens-backups/wal/` from within the database container host.

---

## 4. Key and Secret Rotation

### Overview

SoroLens utilizes multiple classes of credentials and cryptographic secrets:
1. **API Keys** (`sl_...`): Used by clients and bots. Enforce scopes (`read:contracts`, `write:contracts`, `admin:*`) and roles (`viewer`, `contributor`, `admin`).
2. **Webhook Subscription Signing Secrets**: HMAC secrets used to sign webhook alerts dispatched to subscribers.
3. **Session Signing Key (`IDENTITY_SIGNING_KEY`)**: HMAC-SHA256 secret used to mint and verify browser workspace session tokens (`sls_...`).
4. **Report Signing Key (`REPORT_SIGNING_KEY`)**: HMAC key used to generate cryptographic integrity signatures for SLA contract reports.
5. **Slack Signing Secret (`SLACK_SIGNING_SECRET`)**: Verifies inbound requests to `/integrations/slack/commands`.
6. **Infrastructure Credentials**: `DATABASE_URL`, `REDIS_URL`, `COLD_STORAGE_ACCESS_KEY_ID`.

---

### Symptoms

- Credential exposure (API token or secret committed to git or exposed in logs).
- Authentication failures (HTTP 401 Unauthorized / HTTP 403 Forbidden).
- Scheduled security compliance rotation.

---

### Checks and Diagnosis

#### 1. Identify API keys and status
List all active API keys (requires admin API key or admin session token):

```bash
curl -s -H "Authorization: Bearer $ADMIN_API_KEY" \
  http://localhost:8080/api/v1/api-keys | jq .
```

#### 2. Check key usage in audit logs
Inspect audit events to check when the key was last used:

```bash
curl -s -H "Authorization: Bearer $ADMIN_API_KEY" \
  "http://localhost:8080/api/v1/admin/audit?limit=20" | jq .
```

#### 3. List alert subscriptions to inspect webhook signing secrets
```bash
curl -s -H "Authorization: Bearer $ADMIN_API_KEY" \
  http://localhost:8080/api/v1/watchdog/subscriptions | jq .
```

---

### Remediation Commands

#### Procedure A: Rotate an API Key in place
Rotating an API key regenerates its secret token in place. The old token is invalidated immediately, while the key's ID, name, scopes, and user bindings are preserved. `rotated_at` is stamped.

```bash
curl -s -X POST -H "Authorization: Bearer $ADMIN_API_KEY" \
  "http://localhost:8080/api/v1/api-keys/<KEY_ID>/rotate" | jq .
```

Expected response:
```json
{
  "id": "abc123xyz",
  "name": "indexer-bot",
  "scopes": ["read:contracts", "write:contracts"],
  "key": "sl_new_plain_text_token_revealed_once",
  "created_at": "2026-09-01T10:00:00Z",
  "rotated_at": "2026-10-17T12:00:00Z"
}
```
> **Warning**: Store the returned `key` value immediately. SoroLens stores only a SHA-256 hash (`store.HashKey(token)`); the plaintext token cannot be retrieved again.

*(Note: The alias `POST /api/v1/admin/keys/{id}/rotate` is also available).*

#### Procedure B: Revoke an API Key immediately
If a key is compromised and should not be renewed:

```bash
curl -s -X DELETE -H "Authorization: Bearer $ADMIN_API_KEY" \
  "http://localhost:8080/api/v1/api-keys/<KEY_ID>"
```

#### Procedure C: Rotate a Webhook Subscription Signing Secret
Regenerate a webhook subscription's signing secret:

```bash
curl -s -X POST -H "Authorization: Bearer $ADMIN_API_KEY" \
  "http://localhost:8080/api/v1/watchdog/subscriptions/<SUBSCRIPTION_ID>/rotate" | jq .
```
Expected response:
```json
{
  "id": "sub_456",
  "signing_secret": "whsec_...",
  "created_at": "2026-09-01T10:00:00Z",
  "rotated_at": "2026-10-17T12:00:00Z"
}
```
Update the webhook consumer service with the new signing secret immediately.

#### Procedure D: Rotate Server Environment Secrets

##### 1. Session Signing Key (`IDENTITY_SIGNING_KEY`)
- Generate a new 256-bit hex secret:
  ```bash
  openssl rand -hex 32
  ```
- Update `IDENTITY_SIGNING_KEY` in `.env` (or cloud secret manager).
- Perform a rolling restart of the API:
  ```bash
  docker compose restart api
  ```
- *Impact*: All existing browser session tokens (`sls_...`) are invalidated immediately. Users will need to generate a new session via `POST /api/v1/auth/session`.

##### 2. SLA Report Signing Key (`REPORT_SIGNING_KEY`)
- Generate a new secret:
  ```bash
  openssl rand -hex 32
  ```
- Update `REPORT_SIGNING_KEY` in environment.
- Restart the API service:
  ```bash
  docker compose restart api
  ```
- *Impact*: Future reports generated via `GET /api/v1/contracts/{id}/report` will carry signatures verified against the new key.

##### 3. Slack Signing Secret (`SLACK_SIGNING_SECRET`)
- In your Slack App Management Dashboard: **Basic Information → App Credentials → Signing Secret → Regenerate**.
- Update `SLACK_SIGNING_SECRET` in `.env`.
- Restart the API service:
  ```bash
  docker compose restart api
  ```

##### 4. Database Password (`POSTGRES_PASSWORD` / `DATABASE_URL`)
- Update the user password in PostgreSQL:
  ```bash
  docker compose exec postgres psql -U sorolens -d sorolens -c \
    "ALTER USER sorolens WITH PASSWORD 'new_strong_password';"
  ```
- Update `DATABASE_URL` and `DIRECT_DATABASE_URL` in `.env`:
  ```
  DATABASE_URL=postgres://sorolens:new_strong_password@localhost:5432/sorolens?sslmode=disable
  DIRECT_DATABASE_URL=postgres://sorolens:new_strong_password@localhost:5432/sorolens?sslmode=disable
  ```
- Restart API and indexer services:
  ```bash
  docker compose restart api indexer
  ```

---

### Verification

#### 1. Verify old API key rejection
Test that the revoked or rotated old API key returns HTTP 401 Unauthorized:

```bash
curl -s -o /dev/null -w "%{http_code}\n" \
  -H "Authorization: Bearer $OLD_API_KEY" \
  http://localhost:8080/api/v1/contracts
```
Expected output: `401`.

#### 2. Verify new API key acceptance
Test that the new API key succeeds:

```bash
curl -s -o /dev/null -w "%{http_code}\n" \
  -H "Authorization: Bearer $NEW_API_KEY" \
  http://localhost:8080/api/v1/contracts
```
Expected output: `200`.

#### 3. Verify audit log entry
Verify that the key rotation was recorded in the audit trail:

```bash
curl -s -H "Authorization: Bearer $ADMIN_API_KEY" \
  "http://localhost:8080/api/v1/admin/audit?limit=10" | \
  jq '.events[] | select(.action == "api_key.rotated" or .action == "subscription.rotated")'
```

#### 4. Verify API service health
```bash
curl -s http://localhost:8080/health
curl -s http://localhost:8080/readyz
```
Both endpoints should return `{"status":"ok"}`.

---

### Escalation and Rollback Notes

- Key rotations cannot be reversed: once `store.RotateAPIKey` executes, the previous token hash is overwritten.
- If a downstream integration fails due to key mismatch during an incident, immediately mint a new emergency scoped key via `POST /api/v1/api-keys` and supply it to the client.
