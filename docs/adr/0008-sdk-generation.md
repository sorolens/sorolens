# 0008. OpenAPI spec as source of truth and automated SDK generation

## Status

Accepted (retroactive) — 2026-09-24

## Context

Sorolens maintains client libraries across multiple languages (Go, Python, TypeScript, JVM, Rust). Hand-rolling HTTP clients leads to protocol drift, missing endpoint implementations, and delayed updates as the API expands.

## Decision

Establish `docs/openapi.yaml` as the canonical source of truth and automate SDK client generation:

1. **Specification as Source of Truth:** The hand-maintained OpenAPI 3.0.3 specification (`docs/openapi.yaml`) defines all endpoints, parameters, and models matching live Chi router routes.
2. **Automated Client Generation:**
   - **Go client (`packages/go-client`):** Generated using `oapi-codegen` v2.4.1 configured via `packages/go-client/oapi-codegen.yaml` (`make client-go`).
   - **Python SDK (`packages/python-sdk`):** Generated using `openapi-python-client` with Ruff formatting.
   - **JVM SDK (`packages/jvm-sdk`):** Generated using OpenAPI Generator via `generate_jvm_client.sh`.
   - **TypeScript SDK:** Types generated from OpenAPI into `apps/web/lib/openapi.generated.ts`.
3. **Drift and Coverage Enforcement in CI:**
   - CI verifies that checked-in Go client code matches generated output (`git diff --exit-code -- packages/go-client/internal`).
   - An SDK coverage guard (`scripts/check_sdk_coverage.py`, issue #556) inspects all OpenAPI operation IDs against SDK methods, failing CI unless documented in `scripts/sdk_coverage_allowlist.json`.

## Consequences

- **Positive:** Eliminates manual client boilerplate; guarantees client synchronization with API releases; CI prevents untracked API coverage gaps.
- **Negative:** Generator limitations require occasional allowlist exemptions.

## Alternatives considered

- Generating OpenAPI spec from Go code comments: Rejected in favor of hand-maintaining the specification to ensure clean API design and schema control.
- Rationale not documented for choosing different generator tools per language rather than a single multi-target generator CLI.

## Sources

- Issues: [#149](https://github.com/sorolens/sorolens/issues/149), [#556](https://github.com/sorolens/sorolens/issues/556)
- PRs: [#293](https://github.com/sorolens/sorolens/pull/293) (commit `5720370`), [#469](https://github.com/sorolens/sorolens/pull/469) (commit `8b9adfb`), [#679](https://github.com/sorolens/sorolens/pull/679) (commit `b1d2d7b`)
- Tooling: [`Makefile`](https://github.com/sorolens/sorolens/blob/main/Makefile#L136-L150), [`packages/go-client/oapi-codegen.yaml`](https://github.com/sorolens/sorolens/blob/main/packages/go-client/oapi-codegen.yaml), [`scripts/check_sdk_coverage.py`](https://github.com/sorolens/sorolens/blob/main/scripts/check_sdk_coverage.py), [`scripts/sdk_coverage_allowlist.json`](https://github.com/sorolens/sorolens/blob/main/scripts/sdk_coverage_allowlist.json)

## Open questions for maintainers

- Rationale not documented for selecting different generator tools per language (`oapi-codegen`, `openapi-python-client`, OpenAPI Generator) rather than standardizing on one framework.
