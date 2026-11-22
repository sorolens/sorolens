# Sorolens SDK Conformance Test Suite

This directory defines shared conformance test fixtures replayed across all official Sorolens SDKs (Go, TypeScript, Python, and Rust).

## Fixture Format

Each fixture is a JSON file placed in `tests/sdk-conformance/fixtures/` structured as follows:

```json
{
  "id": "01_contract_get_success",
  "description": "Human-readable description of scenario",
  "request": {
    "method": "GET",
    "path": "/api/v1/contracts/{id}"
  },
  "response": {
    "status": 200,
    "headers": {
      "Content-Type": "application/json"
    },
    "body": { ... }
  },
  "expected": {
    "status": 200,
    "type": "contract"
  }
}
```

## Scenarios Covered

1. **Success Envelopes**: Basic single-entity retrieval, global stats, event queries, and interface specs.
2. **Pagination**: Initial page with continuation cursor, subsequent page requests, empty lists (`[]` not `null`).
3. **4xx Client Errors**: 400 Bad Request, 401 Unauthorized, 404 Not Found envelopes.
4. **5xx Server Errors**: 500 Internal Error, 503 Service Unavailable envelopes.
5. **Rate Limiting**: 429 Too Many Requests with `Retry-After` header extraction.
6. **Forward Compatibility**: Response payloads containing unmapped/unknown future fields must be tolerated gracefully without crashing deserialization.

## Running Conformance Tests

All runners run within each SDK's existing test command:

- **Go Client**: `cd packages/go-client && go test -run TestSDKConformance`
- **TypeScript SDK**: `cd packages/typescript-sdk && npm test`
- **Python SDK**: `cd packages/python-sdk && pytest tests/test_conformance.py`
- **Rust SDK**: `cd packages/rust-sdk && cargo test --test conformance`

## Adding New Fixtures

1. Add a new `.json` file under `tests/sdk-conformance/fixtures/` with a numbered prefix (e.g. `17_new_scenario.json`).
2. Run each SDK test suite to verify conformance across all language implementations.
