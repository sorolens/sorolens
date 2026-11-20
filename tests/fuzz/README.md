# OpenAPI Contract Fuzzing

Automated schema-based contract fuzzing against the Sorolens API (issue #850).

Every documented endpoint in `docs/openapi.yaml` is exercised with generated valid, invalid, boundary, and malformed inputs to ensure:
- No unhandled HTTP 500 server crashes occur.
- Status codes conform to HTTP standards and schema specifications.
- Response formats match `Content-Type: application/json`.
- Latency remains bounded (< 5s).

## Excluded Endpoints

A small, justified exclusion list is maintained:
- `/api/v1/stream/events`: Server-Sent Events (SSE) stream maintains a long-lived open connection.
- `/api/v1/subscribe`: WebSocket connection requiring protocol upgrade handshake.
- `/debug/pprof/*`: Go runtime low-level profiling endpoints.
- `/integrations/slack/commands`: Slack webhook endpoint requiring signed Slack HMAC headers.

## Running Locally

### Option 1: Native Go Contract Fuzzer
No additional dependencies required:

```bash
go test -v ./tests/integration/... -run TestOpenAPIContractFuzz
```

The report will be output to `tests/fuzz/reports/fuzz-report.json`.

### Option 2: Schemathesis (Python)

Install Schemathesis:
```bash
pip install schemathesis
```

Start the API locally or via Docker:
```bash
docker compose up -d
make api
```

Run the fuzzer script:
```bash
./tests/fuzz/run_fuzzer.sh
```

JUnit XML and JSON reports are generated in `tests/fuzz/reports/`.

## Scheduled CI Execution

The fuzzer runs automatically in the scheduled integration workflow (`.github/workflows/integration.yml`) on a nightly cron and on `workflow_dispatch`. Results are published as workflow artifacts.
