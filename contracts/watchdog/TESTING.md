# Watchdog Contract Testing & Invariants

This document describes the core invariants verified for the `sorolens-watchdog` Soroban contract, along with instructions for running and extending the property-based and fuzz tests.

## Core Invariants

1. **Registry Consistency**: The number of registered contracts matches total registrations minus deregistrations, and checking if a contract is registered always aligns with the membership state.
2. **Pagination Completeness**: Paginated queries over registered contracts or alerts return all records without omissions or infinite loops across arbitrary page limits and start cursors.
3. **Pause Semantics**: When the watchdog contract is paused, state-mutating operations such as registering contracts, reporting health checks, and filing alerts are successfully blocked with a pause error, while read operations remain functional.
4. **Alert Ordering**: Alerts generated for a contract are stored and retrieved in strictly chronological order (or reverse chronological order depending on query parameters) without duplicate timestamps or missing sequence gaps.
5. **Ownership and Auth**: Administrative actions (such as `pause`, `unpause`, and `register_contract`) strictly require authorization from the designated admin address, rejecting unauthorized invocations.

## Running Tests

To run the watchdog contract test suite including property-based tests:

```bash
cargo test --package sorolens-watchdog
```

To run with fuzzing / proptest regression reporting enabled, use standard cargo test flags or dedicated proptest runners.
