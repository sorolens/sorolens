# 0006. Server-Sent Events (SSE) and WebSocket protocol selection

## Status

Accepted (retroactive) — 2026-09-28

## Context

Sorolens requires real-time streaming for live contract feeds, dashboard charts, the CLI tail command (`sorolens events tail`), and multi-contract watchdog alert feeds. Unidirectional consumption has different performance characteristics than dynamic, client-filtered subscriptions.

## Decision

Support both Server-Sent Events (SSE) and WebSockets according to communication patterns:

1. **Server-Sent Events (SSE) for unidirectional streaming:**
   - Exposed at `GET /api/v1/stream/events` and `GET /api/v1/contracts/{id}/stream`.
   - Streams events over standard HTTP using Go's `http.Flusher`.
   - Supports reconnection with `Last-Event-ID` and periodic heartbeats.
   - Ideal for CLI event tailing, simple script consumers, and read-only live feeds traversing proxies, load balancers, and CDNs without connection upgrade overhead.
2. **WebSockets for interactive subscriptions:**
   - Exposed at `GET /api/v1/subscribe` using `github.com/coder/websocket`.
   - Supports bidirectional messaging (`{"op": "subscribe", "filter": {...}}` and `{"op": "unsubscribe", "id": ...}`).
   - Enables browser clients (`useSubscription` hook) to add, change, or remove subscription filters live over a single persistent connection without reconnecting.
   - Pings every 30s and terminates clients missing heartbeats past 60s.

## Consequences

- **Positive:** Dedicated transport for each pattern: simple HTTP streaming for unidirectional feeds and stateful multiplexing for interactive UI components.
- **Negative:** Two real-time protocols must be maintained, documented in OpenAPI, and tested in the API layer.

## Alternatives considered

- WebSocket-only: Rejected because CLI and basic HTTP clients would require full WebSocket framing for simple event tailing.
- Long polling: Rejected due to excessive HTTP connection churn under high event frequencies.

## Sources

- Issues: [#126](https://github.com/sorolens/sorolens/issues/126), [#339](https://github.com/sorolens/sorolens/issues/339)
- Pull requests: [#339](https://github.com/sorolens/sorolens/pull/339) (commit `55dd719`), [#414](https://github.com/sorolens/sorolens/pull/414) (commit `21cdfac`), [#432](https://github.com/sorolens/sorolens/pull/432) (commit `504f049`)
- Code: [`apps/api/internal/handler/stream_ws.go`](https://github.com/sorolens/sorolens/blob/main/apps/api/internal/handler/stream_ws.go), [`apps/web/hooks/useSubscription.ts`](https://github.com/sorolens/sorolens/blob/main/apps/web/hooks/useSubscription.ts)

## Open questions for maintainers

- Whether a WebSocket subscription endpoint will be exposed under `/api/v2`, or if SSE remains the sole real-time transport for v2.
