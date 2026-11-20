# Integration Tests

This suite proves the product works end to end:

```
mock Soroban RPC (one real event) -> indexer (-mode once) -> Postgres -> API
```

Every assertion is exact. If indexing stops writing rows, the indexing cursor
stops advancing, or the API stops serialising a field, the suite fails — it
never accepts a bare "2xx or 4xx".

## Requirements

- Docker with either the Compose v2 plugin (`docker compose`) or the
  standalone `docker-compose` binary.
- Go 1.26 (for `go test`).

## Run

```sh
cd tests/integration
make integration-test
```

`make integration-test` builds the API, indexer and mock RPC images, starts
Postgres, Redis, the mock RPC, migrations and the API, then runs the Go suite.
The suite registers a contract through `POST /api/v1/contracts`,
runs the indexer once using `docker compose run --rm indexer -mode once`, and
verifies the event, invocation, stats, filters and idempotency through the API.
The stack is torn down (including volumes) afterwards, on success or failure.

The suite can also be run against an already-running stack:

```sh
make up
go test ./... -v -count=1
make down
```

## Fixture

The mock RPC (`mock_rpc/main.go`) serves one event for
`CBIELTK6YBZJU5UP2WWQEUCYKLPU6AUNZ2BQ4WWFEIE3USCIHMXQDAMA` on testnet at ledger
999999 with a real `Symbol("transfer")` topic and `I128(1000)` value. It only
returns the event when the indexer requests the matching contract, event type
and ledger window.

Registration leaves a contract in `pending`. The single indexer pass the suite
runs takes it through `backfilling` to `active`.

## Fault Injection Testing

The integration suite includes fault injection scenarios (issue #810) to verify indexer resilience against transient failures, timeouts, packet slicing, database disconnects, and process termination:

```sh
make fault-test
```

### Available Scenarios (`fault_test.go`)
1. **RPC Timeout Mid-Window (`TestFault_RPCTimeoutMidWindow`):** Injects a 6-second timeout on RPC calls, verifying the indexer retries safely without corrupting the cursor or emitting partial state.
2. **Partial / Truncated Response (`TestFault_RPCPartialResponse`):** Returns severed JSON packets, proving the transaction aborts and cleanly rolls back without saving malformed records.
3. **Database Connection Loss (`TestFault_DatabaseConnectionLossDuringWrite`):** Simulates abrupt database socket resets, ensuring atomic transaction rollback and full recovery once connectivity is restored.
4. **Process Termination & Restart (`TestFault_ProcessKillAndRestartCursorResume`):** Kills indexer mid-window, proving subsequent passes resume from the persisted Postgres cursor idempotently.
5. **Transient 503 / Rate Limiting (`TestFault_TransientRPC503OrRateLimit`):** Simulates HTTP 503 / 429 backoff handling and recovery.

### Adding a New Scenario
1. In `tests/integration/fault_test.go`, create a new test `func TestFault_YourScenario(t *testing.T)`.
2. Configure faults using `NewToxiproxyClient()` for network-level drops/latency or `NewMockRPCFaultClient()` for protocol-level status codes and truncations.
3. Execute the indexer pass with `runIndexerOnce()`.
4. Clear faults with `ResetAllFaults()` and trigger the recovery pass.
5. Assert data completeness and idempotency using `assertDataCompleteness(t, expectedCount)`.

---

## CI

`.github/workflows/integration.yml` runs this suite on every pull request to
`main` and nightly on a schedule.
