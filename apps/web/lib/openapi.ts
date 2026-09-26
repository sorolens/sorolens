/**
 * A compact, OpenAPI-shaped description of the Sorolens REST surface.
 *
 * The playground reads this document to render the selectable endpoint list
 * and the per-endpoint parameter form. It intentionally follows the shape of
 * an OpenAPI 3 document so that, once issue #104 publishes a generated spec,
 * this module can be swapped for a loader with no changes to the playground.
 */

export type HttpMethod = "GET" | "POST" | "DELETE";

export interface OpenAPIParameter {
  name: string;
  in: "path" | "query";
  required?: boolean;
  description?: string;
  type?: "string" | "integer";
  default?: string;
}

export interface OpenAPIRequestBody {
  description?: string;
  example: string;
}

export interface OpenAPIOperation {
  operationId: string;
  summary: string;
  tags: string[];
  parameters: OpenAPIParameter[];
  requestBody?: OpenAPIRequestBody;
}

export interface OpenAPIPathItem {
  get?: OpenAPIOperation;
  post?: OpenAPIOperation;
  delete?: OpenAPIOperation;
}

export interface OpenAPIDocument {
  openapi: string;
  info: { title: string; version: string };
  paths: Record<string, OpenAPIPathItem>;
}

const pathParam = (name: string, description: string): OpenAPIParameter => ({
  name,
  in: "path",
  required: true,
  description,
  type: "string",
});

const contractIdParam = pathParam(
  "id",
  "56-character StrKey contract id starting with C",
);

const networkParam: OpenAPIParameter = {
  name: "network",
  in: "query",
  type: "string",
  description: "Filter by network: testnet | mainnet | futurenet",
};

export const openApiDocument: OpenAPIDocument = {
  openapi: "3.1.0",
  info: { title: "Sorolens API", version: "0.1.0" },
  paths: {
    "/api/v1/stats/global": {
      get: {
        operationId: "getGlobalStats",
        summary: "Global tracked-contract summary",
        tags: ["Stats"],
        parameters: [networkParam],
      },
    },
    "/api/v1/contracts": {
      get: {
        operationId: "listContracts",
        summary: "List tracked contracts",
        tags: ["Contracts"],
        parameters: [
          { name: "cursor", in: "query", description: "Opaque pagination cursor" },
          { name: "limit", in: "query", type: "integer", default: "50" },
          networkParam,
          { name: "status", in: "query", description: "pending | backfilling | active | paused | error" },
        ],
      },
      post: {
        operationId: "registerContract",
        summary: "Register a contract for tracking",
        tags: ["Contracts"],
        parameters: [],
        requestBody: {
          description: "Contract to track",
          example: JSON.stringify(
            {
              id: "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
              network: "testnet",
              label: "My Contract",
            },
            null,
            2,
          ),
        },
      },
    },
    "/api/v1/contracts/{id}": {
      get: {
        operationId: "getContract",
        summary: "Get one contract's metadata",
        tags: ["Contracts"],
        parameters: [contractIdParam],
      },
    },
    "/api/v1/contracts/{id}/events": {
      get: {
        operationId: "listEvents",
        summary: "Paginated event list",
        tags: ["Events"],
        parameters: [
          contractIdParam,
          { name: "cursor", in: "query" },
          { name: "limit", in: "query", type: "integer", default: "50" },
          networkParam,
          { name: "type", in: "query", description: "contract | system" },
          { name: "from", in: "query", type: "integer" },
          { name: "to", in: "query", type: "integer" },
        ],
      },
    },
    "/api/v1/contracts/{id}/invocations": {
      get: {
        operationId: "listInvocations",
        summary: "Paginated invocation list",
        tags: ["Events"],
        parameters: [
          contractIdParam,
          { name: "cursor", in: "query" },
          { name: "limit", in: "query", type: "integer", default: "50" },
          networkParam,
          { name: "status", in: "query", description: "SUCCESS | FAILED | NOT_FOUND" },
          { name: "fn", in: "query", description: "Filter by function name" },
          { name: "from", in: "query", type: "integer" },
          { name: "to", in: "query", type: "integer" },
        ],
      },
    },
    "/api/v1/contracts/{id}/storage": {
      get: {
        operationId: "listStorageEntries",
        summary: "Current storage entries with TTL data",
        tags: ["Storage"],
        parameters: [
          contractIdParam,
          { name: "cursor", in: "query" },
          { name: "limit", in: "query", type: "integer", default: "50" },
          networkParam,
          { name: "durability", in: "query", description: "temporary | persistent | instance" },
          { name: "status", in: "query", description: "live | archived | deleted" },
        ],
      },
    },
    "/api/v1/contracts/{id}/stats": {
      get: {
        operationId: "getContractStats",
        summary: "Per-contract aggregate statistics",
        tags: ["Stats"],
        parameters: [
          contractIdParam,
          { name: "window", in: "query", description: "24h | 7d | 30d", default: "24h" },
        ],
      },
    },
    "/api/v1/contracts/{id}/snapshot": {
      get: {
        operationId: "getContractSnapshot",
        summary: "Replay storage state at a historical ledger",
        tags: ["Storage"],
        parameters: [
          contractIdParam,
          {
            name: "ledger",
            in: "query",
            required: true,
            type: "integer",
            description: "Ledger to replay the contract state at",
          },
        ],
      },
    },
    "/api/v1/contracts/{id}/stream": {
      get: {
        operationId: "streamEvents",
        summary: "Most recent events (polling)",
        tags: ["Events"],
        parameters: [contractIdParam],
      },
    },
    "/api/v1/watchdog/stats": {
      get: {
        operationId: "getWatchdogStats",
        summary: "Watchdog health summary",
        tags: ["Watchdog"],
        parameters: [networkParam],
      },
    },
    "/api/v1/watchdog/alerts": {
      get: {
        operationId: "listWatchdogAlerts",
        summary: "All watchdog alerts",
        tags: ["Watchdog"],
        parameters: [
          { name: "severity", in: "query", description: "Info | Warning | Critical" },
          { name: "limit", in: "query", type: "integer", default: "100" },
          networkParam,
        ],
      },
    },
    "/api/v1/watchdog/contracts": {
      get: {
        operationId: "listMonitoredContracts",
        summary: "List contracts registered with the watchdog",
        tags: ["Watchdog"],
        parameters: [
          { name: "cursor", in: "query" },
          { name: "limit", in: "query", type: "integer", default: "50" },
          networkParam,
        ],
      },
    },
    "/api/v1/watchdog/contracts/{id}": {
      get: {
        operationId: "getMonitoredContract",
        summary: "One monitored contract's current health",
        tags: ["Watchdog"],
        parameters: [pathParam("id", "Contract id registered with the watchdog")],
      },
    },
    "/api/v1/watchdog/contracts/{id}/health": {
      get: {
        operationId: "listHealthChecks",
        summary: "Health-check timeline for a monitored contract",
        tags: ["Watchdog"],
        parameters: [
          pathParam("id", "Contract id registered with the watchdog"),
          { name: "limit", in: "query", type: "integer", default: "100" },
        ],
      },
    },
    "/api/v1/watchdog/contracts/{id}/alerts": {
      get: {
        operationId: "listContractAlerts",
        summary: "Alerts for a monitored contract",
        tags: ["Watchdog"],
        parameters: [
          pathParam("id", "Contract id registered with the watchdog"),
          { name: "severity", in: "query" },
          { name: "limit", in: "query", type: "integer", default: "100" },
        ],
      },
    },
    "/api/v1/api-keys": {
      get: {
        operationId: "listAPIKeys",
        summary: "List API keys (admin:* scope)",
        tags: ["API keys"],
        parameters: [
          { name: "cursor", in: "query" },
          { name: "limit", in: "query", type: "integer", default: "50" },
        ],
      },
      post: {
        operationId: "createAPIKey",
        summary: "Create a scoped API key (admin:* scope)",
        tags: ["API keys"],
        parameters: [],
        requestBody: {
          description: "Key name and scopes",
          example: JSON.stringify(
            { name: "monitoring bot", scopes: ["read:watchdog"] },
            null,
            2,
          ),
        },
      },
    },
    "/api/v1/api-keys/{id}": {
      delete: {
        operationId: "revokeAPIKey",
        summary: "Revoke an API key (admin:* scope)",
        tags: ["API keys"],
        parameters: [pathParam("id", "API key id")],
      },
    },
  },
};

/** A flattened, order-preserving view of the document's operations. */
export interface EndpointEntry {
  method: HttpMethod;
  path: string;
  operation: OpenAPIOperation;
}

export function listEndpoints(
  doc: OpenAPIDocument = openApiDocument,
): EndpointEntry[] {
  const methods: HttpMethod[] = ["GET", "POST", "DELETE"];
  const out: EndpointEntry[] = [];
  for (const [path, item] of Object.entries(doc.paths)) {
    for (const method of methods) {
      const operation = item[method.toLowerCase() as Lowercase<HttpMethod>];
      if (operation) out.push({ method, path, operation });
    }
  }
  return out;
}

export function endpointsByTag(
  doc: OpenAPIDocument = openApiDocument,
): { tag: string; endpoints: EndpointEntry[] }[] {
  const groups = new Map<string, EndpointEntry[]>();
  for (const entry of listEndpoints(doc)) {
    const tag = entry.operation.tags[0] ?? "Other";
    const list = groups.get(tag) ?? [];
    list.push(entry);
    groups.set(tag, list);
  }
  return Array.from(groups, ([tag, endpoints]) => ({ tag, endpoints }));
}
