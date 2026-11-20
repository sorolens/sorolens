# GraphQL Security & Resource Controls

This document details the security model, complexity budgeting, depth limits, and persisted query enforcement for the Sorolens GraphQL endpoint (`/graphql`), implemented under issue #800.

---

## 1. Threat Model & Overview

The GraphQL endpoint exposes flexible queries over contracts, events, invocations, storage entries, and watchdog alerts. Because arbitrary nested queries can trigger unbounded joins or fan-out attacks, the server enforces strict architectural protections before queries touch the database.

| Control | Limit / Behavior | Rejection Code |
| :--- | :--- | :--- |
| **Max Depth Limit** | Default: `10` | `MAX_DEPTH_EXCEEDED` |
| **Complexity Budget** | Default: `5000` per operation | `COMPLEXITY_LIMIT_EXCEEDED` |
| **Max Page Size** | Max `first: 100` | Capped in resolvers & loaders |
| **Persisted Query Allowlist** | Optional `PersistedOnly` mode | `PERSISTED_QUERY_REQUIRED` |
| **Introspection** | Disabled in `PersistedOnly` mode | N/A |

---

## 2. Depth Limit Enforcement

To prevent deeply nested or circular traversal attacks (e.g. `contracts -> events -> contract -> events -> contract -> ...`), the query AST is inspected before execution.

- Any query exceeding the maximum nesting depth (default `10`) is rejected during AST validation.
- The request fails immediately with HTTP 200 and a GraphQL error without performing database or resolver execution.

---

## 3. Operation Complexity Budget

Complexity is calculated per field and weighted by page size:

$$\text{FieldCost} = 1 + (\text{ChildComplexity} \times \min(\text{first}, \text{MaxListLimit}))$$

- **Scalar fields:** Cost = 1.
- **List fields:** Cost scales linearly with the requested `first` argument. For example, `contracts(first: 50) { events(first: 20) { type } }` costs approximately 1,000 points.
- **Alias Amplification Protection:** Replicating fields across multiple aliases (e.g., 80 aliased `contracts(first: 50)` fields) accumulates complexity across each alias and is rejected once total cost exceeds the budget (`5000`).

---

## 4. Page Size Caps

Resolvers and dataloaders enforce `MaxListLimit = 100`:
- When a client supplies `first: 100000`, the resolver internally clamps the limit to `MaxListLimit`.
- Dataloaders batch lookups across siblings into unified SQL `IN (...)` queries.

---

## 5. Automatic Persisted Queries (APQ) & Persisted-Only Mode

For production environments exposed publicly:
- Clients send queries hashed with SHA-256 (`persistedQuery: { version: 1, sha256Hash: "<hash>" }`).
- When `PersistedOnly` is enabled via configuration:
  1. Only pre-registered queries in `internal/graph/persisted/*.graphql` are permitted.
  2. Ad-hoc query strings are rejected with `PERSISTED_QUERY_REQUIRED`.
  3. Schema introspection is automatically disabled.
