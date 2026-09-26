/**
 * Typed client for the Sorolens HTTP API.
 *
 * The bot talks to the same `/api/v1` surface the web dashboard uses. Calls
 * made on behalf of a linked contributor carry that contributor's own API
 * key (`Authorization: Bearer sl_...`) so the API can attribute and scope
 * the request to their identity, plus an `X-User-ID` header which is how the
 * watchlist endpoints key their rows.
 *
 * The client takes an injectable `fetch` so unit tests never touch the
 * network, and never imports the Discord runtime.
 */

/** Error carrying the API's HTTP status and machine-readable code. */
export class ApiError extends Error {
  readonly status: number;
  readonly code?: string;

  constructor(status: number, message: string, code?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

/** Shape returned by `GET /api/v1/contracts/{id}` (fields we consume). */
export interface ContractStatus {
  id: string;
  network: string;
  label: string | null;
  status: string;
  wasmHash: string | null;
  addedAt: string;
}

/** Shape returned by the watchdog alert endpoints. */
export interface ContractAlert {
  contractId: string;
  severity: string;
  message: string;
  ledger: number;
  txHash: string;
  timestamp: string;
}

/** Per-request credentials. The API key wins over the client default. */
export interface ApiCredentials {
  apiKey?: string;
  userId?: string;
}

export interface SorolensApiOptions {
  baseUrl: string;
  /** Shared fallback key (e.g. the bot's admin key) used when a caller has none. */
  apiKey?: string;
  fetchImpl?: typeof fetch;
  /** Request timeout in milliseconds. Defaults to 8000. */
  timeoutMs?: number;
}

export interface ApiKeyGrant {
  id: string;
  key: string;
  keyPrefix: string;
}

/** Scopes granted to each contributor's key. Read the surfaces the bot exposes. */
export const USER_KEY_SCOPES = ["read:contracts", "read:watchdog", "write:contracts"];

/** Contract ids are Stellar contract strkeys: 56 chars starting with `C`. */
export function isValidContractId(id: string): boolean {
  return /^C[A-Z2-7]{55}$/.test(id.trim().toUpperCase());
}

function normalizeBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, "");
}

export class SorolensApi {
  private readonly baseUrl: string;
  private readonly defaultKey: string;
  private readonly fetchImpl: typeof fetch;
  private readonly timeoutMs: number;

  constructor(opts: SorolensApiOptions) {
    this.baseUrl = normalizeBaseUrl(opts.baseUrl);
    this.defaultKey = (opts.apiKey ?? "").trim();
    this.fetchImpl = opts.fetchImpl ?? fetch;
    this.timeoutMs = opts.timeoutMs ?? 8000;
  }

  /** `GET /api/v1/contracts/{id}` — current status of one tracked contract. */
  async getContractStatus(id: string, creds: ApiCredentials = {}): Promise<ContractStatus> {
    const body = await this.request<{
      id: string;
      network?: string;
      label?: string | null;
      status?: string;
      wasm_hash?: string | null;
      added_at?: string;
    }>("GET", `/api/v1/contracts/${encodeURIComponent(id)}`, creds);
    return {
      id: body.id ?? id,
      network: body.network ?? "unknown",
      label: body.label ?? null,
      status: body.status ?? "unknown",
      wasmHash: body.wasm_hash ?? null,
      addedAt: body.added_at ?? "",
    };
  }

  /** `GET /api/v1/watchdog/contracts/{id}/alerts` — most recent alerts first. */
  async listAlerts(
    id: string,
    limit = 5,
    creds: ApiCredentials = {},
  ): Promise<ContractAlert[]> {
    const capped = Math.min(Math.max(limit, 1), 10);
    const body = await this.request<{ alerts?: RawAlert[] }>(
      "GET",
      `/api/v1/watchdog/contracts/${encodeURIComponent(id)}/alerts?limit=${capped}`,
      creds,
    );
    return (body.alerts ?? []).slice(0, capped).map(normalizeAlert);
  }

  /** `GET /api/v1/watchlist` — the caller's watched contract ids. */
  async listWatchlist(creds: ApiCredentials = {}): Promise<string[]> {
    const body = await this.request<{ items?: { contract_id?: string }[] }>(
      "GET",
      "/api/v1/watchlist",
      creds,
    );
    return (body.items ?? [])
      .map((i) => i.contract_id)
      .filter((id): id is string => typeof id === "string" && id.length > 0);
  }

  /** `POST /api/v1/watchlist` — returns whether the contract is now watched. */
  async addToWatchlist(id: string, creds: ApiCredentials = {}): Promise<boolean> {
    const body = await this.request<{ in_watchlist?: boolean }>(
      "POST",
      "/api/v1/watchlist",
      creds,
      { contract_id: id },
    );
    return body.in_watchlist ?? true;
  }

  /** `DELETE /api/v1/watchlist/{id}` — returns whether it is still watched. */
  async removeFromWatchlist(id: string, creds: ApiCredentials = {}): Promise<boolean> {
    const body = await this.request<{ in_watchlist?: boolean }>(
      "DELETE",
      `/api/v1/watchlist/${encodeURIComponent(id)}`,
      creds,
    );
    return body.in_watchlist ?? false;
  }

  /**
   * `POST /api/v1/api-keys` — mint a per-contributor key. Requires the
   * client's default key to carry the `admin:*` scope; the plaintext key is
   * returned exactly once and must be stored by the caller. Used at OAuth
   * link time to give each user their own identity.
   */
  async provisionApiKey(name: string, scopes: string[] = USER_KEY_SCOPES): Promise<ApiKeyGrant> {
    const body = await this.request<{ id: string; key?: string; key_prefix?: string }>(
      "POST",
      "/api/v1/api-keys",
      {},
      { name, scopes },
    );
    if (!body.key) {
      throw new ApiError(502, "API returned no key material for the new API key");
    }
    return { id: body.id, key: body.key, keyPrefix: body.key_prefix ?? "" };
  }

  /** `DELETE /api/v1/api-keys/{id}` — revoke a provisioned key (best effort). */
  async revokeApiKey(id: string): Promise<void> {
    await this.request<void>("DELETE", `/api/v1/api-keys/${encodeURIComponent(id)}`, {});
  }

  private headers(creds: ApiCredentials): Record<string, string> {
    const headers: Record<string, string> = {
      Accept: "application/json",
      "User-Agent": "sorolens-discord-bot",
    };
    const key = (creds.apiKey ?? this.defaultKey).trim();
    if (key) headers.Authorization = `Bearer ${key}`;
    if (creds.userId) headers["X-User-ID"] = creds.userId;
    return headers;
  }

  private async request<T>(
    method: string,
    path: string,
    creds: ApiCredentials,
    body?: unknown,
  ): Promise<T> {
    const headers = this.headers(creds);
    if (body !== undefined) headers["Content-Type"] = "application/json";

    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeoutMs);
    let res: Response;
    try {
      res = await this.fetchImpl(`${this.baseUrl}${path}`, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
        signal: controller.signal,
      });
    } catch (err) {
      throw new ApiError(0, `could not reach Sorolens API: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      clearTimeout(timer);
    }

    if (res.status === 204) return undefined as T;

    const text = await res.text();
    let parsed: unknown = undefined;
    if (text) {
      try {
        parsed = JSON.parse(text);
      } catch {
        parsed = undefined;
      }
    }

    if (!res.ok) {
      const obj = (parsed ?? {}) as { error?: string; code?: string };
      throw new ApiError(res.status, obj.error ?? res.statusText ?? "request failed", obj.code);
    }
    return (parsed ?? {}) as T;
  }
}

interface RawAlert {
  contract_id?: string;
  severity?: string;
  message?: string;
  ledger?: number;
  tx_hash?: string;
  timestamp?: string;
}

function normalizeAlert(a: RawAlert): ContractAlert {
  return {
    contractId: a.contract_id ?? "",
    severity: a.severity ?? "Info",
    message: a.message ?? "(no message)",
    ledger: a.ledger ?? 0,
    txHash: a.tx_hash ?? "",
    timestamp: a.timestamp ?? "",
  };
}
