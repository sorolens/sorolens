import type { OpenAPIDocument } from "./openapi";

export const generatedOpenApiDocument: OpenAPIDocument = {
  openapi: "3.0.3",
  info: {
    title: "Sorolens API",
    version: "1.0.0",
  },
  paths: {
    "/api/v1/invocations/{tx_hash}/trace": {
      get: {
        operationId: "getInvocationTrace",
        summary: "Cross-contract call graph trace for a transaction",
        tags: ["Contracts"],
        parameters: [
          {
            name: "tx_hash",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v1/watchdog/subscriptions/{id}/signing-secret": {
      get: {
        operationId: "getSubscriptionSigningSecret",
        summary: "Reveal a subscription signing secret",
        tags: ["Watchdog"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v1/watchdog/subscriptions/{id}/rotate": {
      post: {
        operationId: "rotateSubscriptionSigningSecret",
        summary: "Rotate a subscription signing secret",
        tags: ["Watchdog"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/health": {
      get: {
        operationId: "health",
        summary: "Service health probe",
        tags: ["Health"],
        parameters: [],
      },
    },
    "/readyz": {
      get: {
        operationId: "readyz",
        summary: "Readiness probe for postgres and redis",
        tags: ["Health"],
        parameters: [],
      },
    },
    "/graphql": {
      post: {
        operationId: "graphql",
        summary: "Read-only GraphQL endpoint",
        tags: ["GraphQL"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/watched-accounts": {
      get: {
        operationId: "listWatchedAccounts",
        summary: "List watched accounts",
        tags: ["Watched Accounts"],
        parameters: [],
      },
      post: {
        operationId: "addWatchedAccount",
        summary: "Watch an account for new contract deployments",
        tags: ["Watched Accounts"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/watched-accounts/{id}": {
      delete: {
        operationId: "deleteWatchedAccount",
        summary: "Stop watching an account",
        tags: ["Watched Accounts"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Stellar account ID to stop watching.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/admin/audit": {
      get: {
        operationId: "listAuditEvents",
        summary: "List audit events (admin role only)",
        tags: ["Audit"],
        parameters: [
          {
            name: "since",
            in: "query",
            required: false,
            description: "Only events at or after this RFC 3339 timestamp.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque cursor from next_cursor of the previous page.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/stats/global": {
      get: {
        operationId: "getGlobalStats",
        summary: "Global index statistics",
        tags: ["Stats"],
        parameters: [],
      },
    },
    "/api/v1/contracts": {
      get: {
        operationId: "listContracts",
        summary: "List tracked contracts",
        tags: ["Contracts"],
        parameters: [
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque pagination cursor from a previous response.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description: "Network filter; empty means all networks.",
            type: "string",
          },
          {
            name: "status",
            in: "query",
            required: false,
            description:
              "Filter by contract status (pending, active, backfilling, error, paused).",
            type: "string",
          },
          {
            name: "tag",
            in: "query",
            required: false,
            description: "Show only contracts carrying this tag.",
            type: "string",
          },
          {
            name: "sort",
            in: "query",
            required: false,
            description:
              "Column to sort by. Defaults to id when omitted; values outside the enum are rejected with 422.",
            type: "string",
          },
          {
            name: "dir",
            in: "query",
            required: false,
            description:
              "Sort direction. Defaults to asc when omitted; values outside the enum are rejected with 422.",
            type: "string",
          },
        ],
      },
      post: {
        operationId: "registerContract",
        summary: "Register a contract for tracking",
        tags: ["Contracts"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/contracts/batch": {
      post: {
        operationId: "batchContracts",
        summary: "Apply a bulk action to many contracts",
        tags: ["Contracts"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/contracts/{id}": {
      get: {
        operationId: "getContract",
        summary: "Get one contract",
        tags: ["Contracts"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/tags": {
      post: {
        operationId: "addContractTag",
        summary: "Add a tag to a contract",
        tags: ["Contracts"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/contracts/{id}/tags/{tag}": {
      delete: {
        operationId: "removeContractTag",
        summary: "Remove a tag from a contract",
        tags: ["Contracts"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "tag",
            in: "path",
            required: true,
            description: "Tag to remove.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/notes": {
      get: {
        operationId: "listContractNotes",
        summary: "List a contract's notes",
        tags: ["Contracts"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
      post: {
        operationId: "createContractNote",
        summary: "Add a note to a contract",
        tags: ["Contracts"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/contracts/{id}/notes/{noteID}": {
      patch: {
        operationId: "updateContractNote",
        summary: "Rewrite a note",
        tags: ["Contracts"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "noteID",
            in: "path",
            required: true,
            description: "Contract note ID.",
            type: "string",
          },
        ],
        requestBody: {
          example: "{}",
        },
      },
      delete: {
        operationId: "deleteContractNote",
        summary: "Delete a note",
        tags: ["Contracts"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "noteID",
            in: "path",
            required: true,
            description: "Contract note ID.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/events": {
      get: {
        operationId: "listContractEvents",
        summary: "List contract events",
        tags: ["Events"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque pagination cursor from a previous response.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description: "Network filter; empty means all networks.",
            type: "string",
          },
          {
            name: "type",
            in: "query",
            required: false,
            description: "Event type filter.",
            type: "string",
          },
          {
            name: "from",
            in: "query",
            required: false,
            description: "Lower ledger bound (inclusive).",
            type: "integer",
          },
          {
            name: "to",
            in: "query",
            required: false,
            description: "Upper ledger bound (inclusive).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/events.csv": {
      get: {
        operationId: "exportContractEventsCSV",
        summary: "Export contract events as CSV",
        tags: ["Events"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description: "Network filter; empty means all networks.",
            type: "string",
          },
          {
            name: "type",
            in: "query",
            required: false,
            description: "Event type filter.",
            type: "string",
          },
          {
            name: "from",
            in: "query",
            required: false,
            description:
              "Lower ledger bound (inclusive). Must not exceed `to`.",
            type: "integer",
          },
          {
            name: "to",
            in: "query",
            required: false,
            description:
              "Upper ledger bound (inclusive). Must not be below `from`.",
            type: "integer",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/invocations": {
      get: {
        operationId: "listContractInvocations",
        summary: "List contract invocations",
        tags: ["Invocations"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque pagination cursor from a previous response.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description: "Network filter; empty means all networks.",
            type: "string",
          },
          {
            name: "status",
            in: "query",
            required: false,
            description: "Invocation status filter.",
            type: "string",
          },
          {
            name: "function_name",
            in: "query",
            required: false,
            description: "Function name filter.",
            type: "string",
          },
          {
            name: "from",
            in: "query",
            required: false,
            description: "Lower ledger bound (inclusive).",
            type: "integer",
          },
          {
            name: "to",
            in: "query",
            required: false,
            description: "Upper ledger bound (inclusive).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/invocations/{tx_hash}": {
      get: {
        operationId: "getContractInvocation",
        summary: "Get contract invocation details with associated events",
        tags: ["Invocations"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "tx_hash",
            in: "path",
            required: true,
            description: "Hex-encoded transaction hash.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/storage": {
      get: {
        operationId: "listContractStorage",
        summary: "List contract storage entries",
        tags: ["Storage"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque pagination cursor from a previous response.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description: "Network filter; empty means all networks.",
            type: "string",
          },
          {
            name: "durability",
            in: "query",
            required: false,
            description: "Storage durability filter.",
            type: "string",
          },
          {
            name: "status",
            in: "query",
            required: false,
            description: "Storage entry status filter.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/stats": {
      get: {
        operationId: "getContractStats",
        summary: "Per-contract statistics",
        tags: ["Stats"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "window",
            in: "query",
            required: false,
            description:
              "Aggregation window. 7d and 30d map to 7/30 days; anything else is 24 hours.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/forecast": {
      get: {
        operationId: "getContractForecast",
        summary: "Forecast contract usage over the next N days",
        tags: ["Forecast"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "horizon",
            in: "query",
            required: false,
            description:
              "Number of days to forecast (default 30; optional trailing d accepted).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/snapshot": {
      get: {
        operationId: "getContractSnapshot",
        summary: "Snapshot of contract storage at a historical ledger",
        tags: ["Snapshots"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "ledger",
            in: "query",
            required: true,
            description: "Ledger to snap (greater than zero).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/verify": {
      post: {
        operationId: "verifyContractSource",
        summary: "Verify contract source against the on-chain Wasm hash",
        tags: ["Verification"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/contracts/{id}/verification": {
      get: {
        operationId: "getContractVerification",
        summary: "Read the stored source verification for a contract",
        tags: ["Verification"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/stream": {
      get: {
        operationId: "streamContractEvents",
        summary: "Poll the most recent contract events",
        tags: ["Events"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/api-keys": {
      get: {
        operationId: "listApiKeys",
        summary: "List API keys",
        tags: ["API Keys"],
        parameters: [
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque pagination cursor from a previous response.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
        ],
      },
      post: {
        operationId: "createApiKey",
        summary: "Create an API key",
        tags: ["API Keys"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/api-keys/{id}": {
      get: {
        operationId: "getApiKey",
        summary: "Get an API key",
        tags: ["API Keys"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "API key ID.",
            type: "string",
          },
        ],
      },
      delete: {
        operationId: "revokeApiKey",
        summary: "Revoke an API key",
        tags: ["API Keys"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "API key ID.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/api-keys/{id}/rotate": {
      post: {
        operationId: "rotateApiKey",
        summary: "Rotate an API key",
        tags: ["API Keys"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "API key ID.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/admin/keys": {
      get: {
        operationId: "listApiKeysAdmin",
        summary: "List API keys (admin role only)",
        tags: ["API Keys"],
        parameters: [
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque pagination cursor from a previous response.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
        ],
      },
      post: {
        operationId: "createApiKeyAdmin",
        summary: "Create an API key (admin role only)",
        tags: ["API Keys"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/admin/keys/{id}": {
      get: {
        operationId: "getApiKeyAdmin",
        summary: "Get an API key (admin role only)",
        tags: ["API Keys"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
      delete: {
        operationId: "revokeApiKeyAdmin",
        summary: "Revoke an API key (admin role only)",
        tags: ["API Keys"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v1/admin/keys/{id}/rotate": {
      post: {
        operationId: "rotateApiKeyAdmin",
        summary: "Rotate an API key (admin role only)",
        tags: ["API Keys"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v1/watchlist": {
      get: {
        operationId: "listWatchlist",
        summary: "List the caller's watchlist",
        tags: ["Watchlist"],
        parameters: [],
      },
      post: {
        operationId: "addToWatchlist",
        summary: "Add a contract to the caller's watchlist",
        tags: ["Watchlist"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/watchlist/{contractId}": {
      delete: {
        operationId: "removeFromWatchlist",
        summary: "Remove a contract from the caller's watchlist",
        tags: ["Watchlist"],
        parameters: [
          {
            name: "contractId",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v1/watchlist/{contractId}/status": {
      get: {
        operationId: "getWatchlistStatus",
        summary: "Check watchlist membership for a contract",
        tags: ["Watchlist"],
        parameters: [
          {
            name: "contractId",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v1/groups": {
      get: {
        operationId: "listGroups",
        summary: "List the caller's contract groups with aggregate statistics",
        tags: ["Groups"],
        parameters: [],
      },
      post: {
        operationId: "createGroup",
        summary: "Create a contract group",
        tags: ["Groups"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/groups/{id}": {
      get: {
        operationId: "getGroup",
        summary: "Get a group with its member contracts",
        tags: ["Groups"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Group ID.",
            type: "string",
          },
        ],
      },
      patch: {
        operationId: "updateGroup",
        summary: "Rename a group",
        tags: ["Groups"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Group ID.",
            type: "string",
          },
        ],
        requestBody: {
          example: "{}",
        },
      },
      delete: {
        operationId: "deleteGroup",
        summary: "Delete a group and its memberships",
        tags: ["Groups"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Group ID.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/groups/{id}/contracts": {
      post: {
        operationId: "addGroupContract",
        summary: "Add a contract to a group",
        tags: ["Groups"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Group ID.",
            type: "string",
          },
        ],
        requestBody: {
          example: "{}",
        },
      },
      delete: {
        operationId: "removeGroupContract",
        summary: "Remove a contract from a group",
        tags: ["Groups"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Group ID.",
            type: "string",
          },
        ],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/groups/{id}/contracts/{contractId}": {
      delete: {
        operationId: "removeGroupContractByPath",
        summary: "Remove a contract from a group (path form)",
        tags: ["Groups"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Group ID.",
            type: "string",
          },
          {
            name: "contractId",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v1/groups/{id}/stats": {
      get: {
        operationId: "getGroupStats",
        summary: "Aggregate statistics across a group's contracts",
        tags: ["Groups"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Group ID.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/watchdog/stats": {
      get: {
        operationId: "getWatchdogStats",
        summary: "Watchdog statistics",
        tags: ["Watchdog"],
        parameters: [
          {
            name: "network",
            in: "query",
            required: false,
            description: "Network filter; empty means all networks.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/watchdog/alerts": {
      get: {
        operationId: "listWatchdogAlerts",
        summary: "List watchdog alerts",
        tags: ["Watchdog"],
        parameters: [
          {
            name: "severity",
            in: "query",
            required: false,
            description: "Severity filter.",
            type: "string",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description: "Network filter; empty means all networks.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Maximum alerts to return (default 100).",
            type: "integer",
          },
          {
            name: "cursor",
            in: "query",
            required: false,
            description:
              "Opaque alert cursor from a previous page's next_cursor.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/watchdog/contracts": {
      get: {
        operationId: "listMonitoredContracts",
        summary: "List monitored contracts",
        tags: ["Watchdog"],
        parameters: [
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque pagination cursor from a previous response.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description: "Network filter; empty means all networks.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/watchdog/contracts/{id}": {
      get: {
        operationId: "getMonitoredContract",
        summary: "Get one monitored contract",
        tags: ["Watchdog"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/watchdog/contracts/{id}/health": {
      get: {
        operationId: "listContractHealthChecks",
        summary: "List health checks for a monitored contract",
        tags: ["Watchdog"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Maximum checks to return (default 100).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v1/watchdog/contracts/{id}/alerts": {
      get: {
        operationId: "listContractAlerts",
        summary: "List alerts for a monitored contract",
        tags: ["Watchdog"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "severity",
            in: "query",
            required: false,
            type: "string",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description: "Network filter; empty means all networks.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            type: "integer",
          },
          {
            name: "cursor",
            in: "query",
            required: false,
            description:
              "Opaque alert cursor from a previous page's next_cursor.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/events/recent": {
      get: {
        operationId: "recentEvents",
        summary: "Newest events across every tracked contract",
        tags: ["Live"],
        parameters: [
          {
            name: "limit",
            in: "query",
            required: false,
            type: "integer",
            default: "50",
          },
        ],
      },
    },
    "/api/v1/stats/activity": {
      get: {
        operationId: "liveActivity",
        summary: "Per-contract events per minute (sparklines and leaderboard)",
        tags: ["Live"],
        parameters: [
          {
            name: "minutes",
            in: "query",
            required: false,
            type: "integer",
            default: "30",
          },
        ],
      },
    },
    "/api/v1/contracts/validate": {
      post: {
        operationId: "validateContract",
        summary: "Validate a contract id before tracking it",
        tags: ["Contracts"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v2/stats/global": {
      get: {
        operationId: "v2GlobalStats",
        summary: "Network-wide summary",
        tags: ["API v2"],
        parameters: [],
      },
    },
    "/api/v2/stats/activity": {
      get: {
        operationId: "v2LiveActivity",
        summary: "Per-contract events per minute",
        tags: ["API v2"],
        parameters: [
          {
            name: "minutes",
            in: "query",
            required: false,
            description: "Window size in one-minute buckets.",
            type: "integer",
            default: "30",
          },
        ],
      },
    },
    "/api/v2/events/recent": {
      get: {
        operationId: "v2RecentEvents",
        summary: "Newest events across every tracked contract",
        tags: ["API v2"],
        parameters: [
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v2/contracts": {
      get: {
        operationId: "v2ListContracts",
        summary: "List tracked contracts",
        tags: ["API v2"],
        parameters: [
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque cursor taken from `pagination.next_cursor`.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description:
              "Network filter; omitted or `all` means every network.",
            type: "string",
          },
          {
            name: "status",
            in: "query",
            required: false,
            description: "Status filter, e.g. `active`.",
            type: "string",
          },
        ],
      },
      post: {
        operationId: "v2RegisterContract",
        summary: "Register a contract for tracking",
        tags: ["API v2"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v2/contracts/validate": {
      post: {
        operationId: "v2ValidateContract",
        summary: "Validate a contract id before tracking it",
        tags: ["API v2"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v2/contracts/{id}": {
      get: {
        operationId: "v2GetContract",
        summary: "Get one contract",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v2/contracts/{id}/events": {
      get: {
        operationId: "v2ListEvents",
        summary: "List a contract's events",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque cursor taken from `pagination.next_cursor`.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description:
              "Network filter; omitted or `all` means every network.",
            type: "string",
          },
          {
            name: "type",
            in: "query",
            required: false,
            type: "string",
          },
          {
            name: "from",
            in: "query",
            required: false,
            type: "integer",
          },
          {
            name: "to",
            in: "query",
            required: false,
            type: "integer",
          },
        ],
      },
    },
    "/api/v2/contracts/{id}/invocations": {
      get: {
        operationId: "v2ListInvocations",
        summary: "List a contract's invocations",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque cursor taken from `pagination.next_cursor`.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description:
              "Network filter; omitted or `all` means every network.",
            type: "string",
          },
          {
            name: "status",
            in: "query",
            required: false,
            type: "string",
          },
          {
            name: "function_name",
            in: "query",
            required: false,
            description: "Renamed from v1's `fn`; `fn` is still accepted.",
            type: "string",
          },
          {
            name: "from",
            in: "query",
            required: false,
            type: "integer",
          },
          {
            name: "to",
            in: "query",
            required: false,
            type: "integer",
          },
        ],
      },
    },
    "/api/v2/contracts/{id}/storage": {
      get: {
        operationId: "v2ListStorageEntries",
        summary: "List a contract's storage entries",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque cursor taken from `pagination.next_cursor`.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description:
              "Network filter; omitted or `all` means every network.",
            type: "string",
          },
          {
            name: "durability",
            in: "query",
            required: false,
            type: "string",
          },
          {
            name: "status",
            in: "query",
            required: false,
            type: "string",
          },
        ],
      },
    },
    "/api/v2/contracts/{id}/stats": {
      get: {
        operationId: "v2ContractStats",
        summary: "Per-contract statistics",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "window",
            in: "query",
            required: false,
            type: "string",
            default: "24h",
          },
        ],
      },
    },
    "/api/v2/contracts/{id}/upgrades": {
      get: {
        operationId: "v2ListContractUpgrades",
        summary: "Wasm-hash upgrade history",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v2/contracts/{id}/health-score": {
      get: {
        operationId: "v2GetContractHealthScore",
        summary: "Cached composite health score",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v2/contracts/{id}/forecast": {
      get: {
        operationId: "v2ContractForecast",
        summary: "Cost and usage forecast (v1 shape)",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "horizon",
            in: "query",
            required: false,
            type: "string",
            default: "30d",
          },
        ],
      },
    },
    "/api/v2/contracts/{id}/snapshot": {
      get: {
        operationId: "v2ContractSnapshot",
        summary: "Storage snapshot at a ledger (v1 shape)",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "ledger",
            in: "query",
            required: true,
            type: "integer",
          },
        ],
      },
    },
    "/api/v2/contracts/{id}/stream": {
      get: {
        operationId: "v2StreamEvents",
        summary: "The 20 newest events, bounded (v1 shape)",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v2/contracts/{id}/graph": {
      get: {
        operationId: "v2ContractGraph",
        summary: "Contract dependency graph (v1 shape)",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v2/watchlist": {
      get: {
        operationId: "v2ListWatchlist",
        summary: "List the caller's watchlist",
        tags: ["API v2"],
        parameters: [],
      },
      post: {
        operationId: "v2AddToWatchlist",
        summary: "Add a contract to the caller's watchlist",
        tags: ["API v2"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v2/watchlist/{contractId}": {
      delete: {
        operationId: "v2RemoveFromWatchlist",
        summary: "Remove a contract from the caller's watchlist",
        tags: ["API v2"],
        parameters: [
          {
            name: "contractId",
            in: "path",
            required: true,
            description: "Contract ID in the watchlist path.",
            type: "string",
          },
        ],
      },
    },
    "/api/v2/watchlist/{contractId}/status": {
      get: {
        operationId: "v2WatchlistStatus",
        summary: "Whether a contract is in the caller's watchlist",
        tags: ["API v2"],
        parameters: [
          {
            name: "contractId",
            in: "path",
            required: true,
            description: "Contract ID in the watchlist path.",
            type: "string",
          },
        ],
      },
    },
    "/api/v2/watchdog/stats": {
      get: {
        operationId: "v2WatchdogStats",
        summary: "Watchdog monitoring summary",
        tags: ["API v2"],
        parameters: [
          {
            name: "network",
            in: "query",
            required: false,
            description:
              "Network filter; omitted or `all` means every network.",
            type: "string",
          },
        ],
      },
    },
    "/api/v2/watchdog/alerts": {
      get: {
        operationId: "v2ListWatchdogAlerts",
        summary: "All watchdog alerts",
        tags: ["API v2"],
        parameters: [
          {
            name: "network",
            in: "query",
            required: false,
            description:
              "Network filter; omitted or `all` means every network.",
            type: "string",
          },
          {
            name: "severity",
            in: "query",
            required: false,
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v2/watchdog/contracts": {
      get: {
        operationId: "v2ListMonitoredContracts",
        summary: "List monitored contracts",
        tags: ["API v2"],
        parameters: [
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque cursor taken from `pagination.next_cursor`.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description:
              "Network filter; omitted or `all` means every network.",
            type: "string",
          },
        ],
      },
    },
    "/api/v2/watchdog/contracts/{id}": {
      get: {
        operationId: "v2GetMonitoredContract",
        summary: "Get one monitored contract",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v2/watchdog/contracts/{id}/health": {
      get: {
        operationId: "v2ListHealthChecks",
        summary: "Health checks for a monitored contract",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v2/watchdog/contracts/{id}/alerts": {
      get: {
        operationId: "v2ListContractAlerts",
        summary: "Alerts for one monitored contract",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50, clamped to 200).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v2/api-keys": {
      get: {
        operationId: "v2ListAPIKeys",
        summary: "List API key metadata (admin)",
        tags: ["API v2"],
        parameters: [],
      },
      post: {
        operationId: "v2CreateAPIKey",
        summary: "Create an API key (admin)",
        tags: ["API v2"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v2/api-keys/{id}": {
      get: {
        operationId: "v2GetAPIKey",
        summary: "Get an API key (admin)",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
      delete: {
        operationId: "v2RevokeAPIKey",
        summary: "Revoke an API key (admin)",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v2/api-keys/{id}/rotate": {
      post: {
        operationId: "v2RotateAPIKey",
        summary: "Rotate an API key (admin)",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v2/admin/keys": {
      get: {
        operationId: "v2AdminListKeys",
        summary: "List API keys (admin role, v1 shape)",
        tags: ["API v2"],
        parameters: [],
      },
      post: {
        operationId: "v2AdminCreateKey",
        summary: "Create an API key (admin role, v1 shape)",
        tags: ["API v2"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v2/admin/keys/{id}": {
      get: {
        operationId: "v2AdminGetKey",
        summary: "Get an API key (admin role, v1 shape)",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
      delete: {
        operationId: "v2AdminRevokeKey",
        summary: "Revoke an API key (admin role, v1 shape)",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v2/admin/keys/{id}/rotate": {
      post: {
        operationId: "v2AdminRotateKey",
        summary: "Rotate an API key (admin role, v1 shape)",
        tags: ["API v2"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v1/dlq": {
      get: {
        operationId: "listFailedEvents",
        summary: "List dead-lettered indexer events",
        tags: ["DLQ"],
        parameters: [
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque pagination cursor from a previous response.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Page size (default 50).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v1/dlq/{id}/requeue": {
      post: {
        operationId: "requeueFailedEvent",
        summary: "Requeue a dead-lettered event",
        tags: ["DLQ"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "The DLQ row's surrogate id.",
            type: "integer",
          },
        ],
      },
    },
    "/api/v1/labels": {
      get: {
        operationId: "get_/api/v1/labels",
        summary: "Search visible account labels",
        tags: ["Other"],
        parameters: [
          {
            name: "query",
            in: "query",
            required: false,
            type: "string",
          },
        ],
      },
      post: {
        operationId: "post_/api/v1/labels",
        summary: "Create or update an account label",
        tags: ["Other"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/resolve": {
      get: {
        operationId: "get_/api/v1/resolve",
        summary: "Resolve a visible label to its Stellar identifier",
        tags: ["Other"],
        parameters: [
          {
            name: "query",
            in: "query",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v1/health": {
      get: {
        operationId: "apiHealth",
        summary: "Health probe with postgres and redis reachability",
        tags: ["Health"],
        parameters: [],
      },
    },
    "/api/v1/auth/session": {
      post: {
        operationId: "createSession",
        summary: "Mint a workspace session token",
        tags: ["Auth"],
        parameters: [],
      },
    },
    "/api/version": {
      get: {
        operationId: "getVersion",
        summary: "Build metadata for the running API instance",
        tags: ["Version"],
        parameters: [],
      },
    },
    "/metrics": {
      get: {
        operationId: "getMetrics",
        summary: "Prometheus metrics",
        tags: ["Operations"],
        parameters: [],
      },
    },
    "/integrations/slack/commands": {
      post: {
        operationId: "slackCommand",
        summary: "Slack slash command",
        tags: ["Integrations"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/events": {
      get: {
        operationId: "listAllEvents",
        summary: "List events across all tracked contracts",
        tags: ["Events"],
        parameters: [
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque pagination cursor from a previous response.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            type: "integer",
            default: "50",
          },
          {
            name: "contract_id",
            in: "query",
            required: false,
            description:
              "Contract ID prefix (case-insensitive); a full ID matches one contract.",
            type: "string",
          },
          {
            name: "type",
            in: "query",
            required: false,
            type: "string",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description: "Network filter; empty means all networks.",
            type: "string",
          },
          {
            name: "since",
            in: "query",
            required: false,
            description:
              "Inclusive lower bound on ledger_closed_at (RFC 3339).",
            type: "string",
          },
          {
            name: "until",
            in: "query",
            required: false,
            description:
              "Inclusive upper bound on ledger_closed_at (RFC 3339).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/invocations": {
      get: {
        operationId: "listAllInvocations",
        summary: "List invocations across all tracked contracts",
        tags: ["Invocations"],
        parameters: [
          {
            name: "cursor",
            in: "query",
            required: false,
            description: "Opaque pagination cursor from a previous response.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            type: "integer",
            default: "50",
          },
          {
            name: "contract_id",
            in: "query",
            required: false,
            description: "Filter to a specific contract ID.",
            type: "string",
          },
          {
            name: "fn",
            in: "query",
            required: false,
            description: "Function name filter.",
            type: "string",
          },
          {
            name: "status",
            in: "query",
            required: false,
            description: "Invocation status filter.",
            type: "string",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description: "Network filter; empty means all networks.",
            type: "string",
          },
          {
            name: "from",
            in: "query",
            required: false,
            description: "Lower ledger bound (inclusive).",
            type: "integer",
          },
          {
            name: "to",
            in: "query",
            required: false,
            description: "Upper ledger bound (inclusive).",
            type: "integer",
          },
          {
            name: "since",
            in: "query",
            required: false,
            description:
              "Inclusive lower bound on ledger_closed_at (RFC 3339).",
            type: "string",
          },
          {
            name: "until",
            in: "query",
            required: false,
            description:
              "Inclusive upper bound on ledger_closed_at (RFC 3339).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/search": {
      get: {
        operationId: "get_/api/v1/search",
        summary: "Search contracts",
        tags: ["Contracts"],
        parameters: [
          {
            name: "q",
            in: "query",
            required: true,
            description: "Search query",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Maximum number of results",
            type: "integer",
            default: "10",
          },
        ],
      },
    },
    "/api/v1/compare": {
      get: {
        operationId: "compareContracts",
        summary: "Compare up to four contracts side by side",
        tags: ["Stats"],
        parameters: [
          {
            name: "ids",
            in: "query",
            required: true,
            description: "1 to 4 comma-separated contract IDs.",
            type: "string",
          },
          {
            name: "window",
            in: "query",
            required: false,
            type: "string",
            default: "24h",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/snapshot.json": {
      get: {
        operationId: "exportContractSnapshot",
        summary: "Portable JSON export of a contract's current state",
        tags: ["Snapshots"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/upgrades": {
      get: {
        operationId: "listContractUpgrades",
        summary: "List Wasm code upgrades for a contract",
        tags: ["Contracts"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            type: "integer",
            default: "50",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/health-score": {
      get: {
        operationId: "getContractHealthScore",
        summary: "Get a contract's composite 0-100 health score",
        tags: ["Stats"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/summary": {
      get: {
        operationId: "getContractSummary",
        summary: "Get a contract's dashboard summary",
        tags: ["Stats"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/graph": {
      get: {
        operationId: "getContractGraph",
        summary: "Get the contract's cross-contract call graph",
        tags: ["Contracts"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/spec": {
      get: {
        operationId: "getContractSpec",
        summary: "Get a contract's parsed SEP-48 interface spec",
        tags: ["Contracts"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/wasm": {
      get: {
        operationId: "getContractWasm",
        summary: "Download the contract's cached Wasm binary",
        tags: ["Contracts"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/stream/events": {
      get: {
        operationId: "streamAllEvents",
        summary: "Stream new events (Server-Sent Events)",
        tags: ["Events"],
        parameters: [
          {
            name: "contract_id",
            in: "query",
            required: false,
            description: "Only stream events for this contract.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/subscribe": {
      get: {
        operationId: "subscribeEventsWS",
        summary: "Subscribe to real-time events (WebSocket)",
        tags: ["Events"],
        parameters: [],
      },
    },
    "/api/v1/watchdog/contracts/{id}/uptime": {
      get: {
        operationId: "getContractUptime",
        summary: "Get uptime percentage for a monitored contract",
        tags: ["Watchdog"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "window",
            in: "query",
            required: false,
            description:
              "Time window over which to compute uptime (default 24h).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/watchdog/subscriptions": {
      get: {
        operationId: "listAlertSubscriptions",
        summary: "List alert notification subscriptions",
        tags: ["Notifications"],
        parameters: [],
      },
      post: {
        operationId: "createAlertSubscription",
        summary: "Subscribe a channel to a contract's alerts",
        tags: ["Notifications"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/watchdog/subscriptions/{id}": {
      delete: {
        operationId: "deleteAlertSubscription",
        summary: "Delete an alert subscription",
        tags: ["Notifications"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v1/alerts": {
      get: {
        operationId: "listAlerts",
        summary: "List deduplicated alert groups",
        tags: ["Alerts"],
        parameters: [
          {
            name: "flat",
            in: "query",
            required: false,
            description:
              "Return the raw ContractAlert feed instead of grouped alerts.",
            type: "string",
          },
          {
            name: "contract_id",
            in: "query",
            required: false,
            description: "Filter by contract ID.",
            type: "string",
          },
          {
            name: "severity",
            in: "query",
            required: false,
            description: "Severity filter.",
            type: "string",
          },
          {
            name: "network",
            in: "query",
            required: false,
            description: "Network filter; empty means all networks.",
            type: "string",
          },
          {
            name: "cursor",
            in: "query",
            required: false,
            description:
              "Opaque alert cursor from a previous page's next_cursor.",
            type: "string",
          },
          {
            name: "limit",
            in: "query",
            required: false,
            description: "Maximum results to return (default 100).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v1/reports/subscriptions": {
      get: {
        operationId: "listReportSubscriptions",
        summary: "List a subscriber's digest subscriptions",
        tags: ["Reports"],
        parameters: [
          {
            name: "email",
            in: "query",
            required: true,
            type: "string",
          },
        ],
      },
      post: {
        operationId: "createReportSubscription",
        summary: "Subscribe to an email digest",
        tags: ["Reports"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/reports/subscriptions/{id}": {
      delete: {
        operationId: "deleteReportSubscription",
        summary: "Delete a digest subscription",
        tags: ["Reports"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v1/reports/unsubscribe": {
      get: {
        operationId: "unsubscribeReport",
        summary: "One-click unsubscribe from digest emails",
        tags: ["Reports"],
        parameters: [
          {
            name: "token",
            in: "query",
            required: true,
            type: "string",
          },
        ],
      },
    },
    "/api/v1/reports/{contract_id}": {
      get: {
        operationId: "getContractReport",
        summary: "Get a contract's monthly SLA report",
        tags: ["Reports"],
        parameters: [
          {
            name: "contract_id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "month",
            in: "query",
            required: false,
            description:
              "Reporting month as YYYY-MM (UTC). Defaults to the current month.",
            type: "string",
          },
          {
            name: "format",
            in: "query",
            required: false,
            description: "Export format (default json).",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/reports/{contract_id}/history": {
      get: {
        operationId: "getContractReportHistory",
        summary: "Get a contract's SLA history across months",
        tags: ["Reports"],
        parameters: [
          {
            name: "contract_id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "months",
            in: "query",
            required: false,
            description: "Number of trailing months to include (default 12).",
            type: "integer",
          },
        ],
      },
    },
    "/api/v1/reports/{contract_id}/badge.svg": {
      get: {
        operationId: "getContractSLABadge",
        summary: "Get an SLA badge SVG for a contract",
        tags: ["Reports"],
        parameters: [
          {
            name: "contract_id",
            in: "path",
            required: true,
            description: "Contract ID (starts with C).",
            type: "string",
          },
          {
            name: "month",
            in: "query",
            required: false,
            description:
              "Reporting month as YYYY-MM (UTC). Defaults to the current month.",
            type: "string",
          },
        ],
      },
    },
    "/api/v1/rules": {
      get: {
        operationId: "listAlertRules",
        summary: "List alert rules",
        tags: ["Alert rules"],
        parameters: [],
      },
      post: {
        operationId: "createAlertRule",
        summary: "Create an alert rule",
        tags: ["Alert rules"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/rules/validate": {
      post: {
        operationId: "validateAlertRule",
        summary: "Validate a rule without storing it",
        tags: ["Alert rules"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/rules/preview": {
      post: {
        operationId: "previewAlertRule",
        summary: "Evaluate a rule against live metric samples",
        tags: ["Alert rules"],
        parameters: [],
        requestBody: {
          example: "{}",
        },
      },
    },
    "/api/v1/rules/metrics": {
      get: {
        operationId: "listRuleMetrics",
        summary: "Rule metric catalog",
        tags: ["Alert rules"],
        parameters: [],
      },
    },
    "/api/v1/rules/library": {
      get: {
        operationId: "listRuleLibrary",
        summary: "Curated sample rules",
        tags: ["Alert rules"],
        parameters: [],
      },
    },
    "/api/v1/rules/{id}": {
      patch: {
        operationId: "setAlertRuleEnabled",
        summary: "Enable or pause a rule",
        tags: ["Alert rules"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "integer",
          },
        ],
        requestBody: {
          example: "{}",
        },
      },
      delete: {
        operationId: "deleteAlertRule",
        summary: "Delete a rule",
        tags: ["Alert rules"],
        parameters: [
          {
            name: "id",
            in: "path",
            required: true,
            type: "integer",
          },
        ],
      },
    },
  },
};
