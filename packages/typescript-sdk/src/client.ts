/**
 * Fetch-only client for the Sorolens API.
 *
 * The module uses nothing outside Web standards (`fetch`, `AbortController`,
 * `URL`, `TextDecoder`, `setTimeout`), so the same build runs on Cloudflare
 * Workers, Deno, Bun, Node 18+ and in the browser.
 */

export type JsonObject = Record<string, unknown>;

export interface SorolensErrorBody {
  status: number;
  code?: string;
  message: string;
  body: string;
}

/** A non-2xx response. `permanent` responses (401/403/other 4xx) are not retried. */
export class SorolensError extends Error {
  readonly status: number;
  readonly code: string | undefined;
  readonly body: string;

  constructor({ status, code, message, body }: SorolensErrorBody) {
    super(message);
    this.name = "SorolensError";
    this.status = status;
    this.body = body;
    this.code = code;
  }

  get permanent(): boolean {
    return this.status >= 400 && this.status < 500 && this.status !== 408 && this.status !== 429;
  }
}

export interface RetryOptions {
  /** Number of retries after the first attempt. Default 2. */
  maxRetries?: number;
  /** Base delay for exponential backoff, in milliseconds. Default 500. */
  baseDelayMs?: number;
  /** Upper bound for any single delay, in milliseconds. Default 30000. */
  maxDelayMs?: number;
  /** Statuses that are retried for safe (GET) requests. */
  retryStatuses?: readonly number[];
}

export interface ClientOptions {
  baseUrl?: string;
  apiKey?: string;
  /** Override `fetch` (tests, custom runtimes). Defaults to `globalThis.fetch`. */
  fetch?: typeof fetch;
  retry?: RetryOptions | false;
  timeoutMs?: number;
}

export interface ListParams {
  cursor?: string;
  limit?: number;
  [key: string]: string | number | undefined;
}

export interface Page<T> {
  items: T[];
  nextCursor?: string;
}

export interface EventEnvelope {
  id?: string;
  type?: string;
  data: unknown;
}

export interface StreamOptions {
  contractId?: string;
  lastEventId?: string;
  signal?: AbortSignal;
  heartbeatTimeoutMs?: number;
  initialBackoffMs?: number;
  maxBackoffMs?: number;
}

function parseJson(payload: string): unknown {
  try {
    return JSON.parse(payload);
  } catch {
    return payload;
  }
}

const DEFAULT_BASE_URL = "https://api.sorolens.xyz";
const DEFAULT_RETRY: Required<Omit<RetryOptions, "retryStatuses">> & {
  retryStatuses: readonly number[];
} = {
  maxRetries: 2,
  baseDelayMs: 500,
  maxDelayMs: 30_000,
  retryStatuses: [429, 500, 502, 503, 504],
};

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function parseRetryAfter(value: string | null): number | undefined {
  if (!value) return undefined;
  const seconds = Number(value);
  if (Number.isFinite(seconds)) return Math.max(seconds, 0) * 1000;
  const when = Date.parse(value);
  if (Number.isNaN(when)) return undefined;
  return Math.max(when - Date.now(), 0);
}

export class SorolensClient {
  private readonly baseUrl: string;
  private readonly apiKey: string | undefined;
  private readonly fetchImpl: typeof fetch;
  private readonly retry: RetryOptions | false;
  private readonly timeoutMs: number;

  constructor(options: ClientOptions = {}) {
    this.baseUrl = (options.baseUrl ?? DEFAULT_BASE_URL).replace(/\/+$/, "");
    this.apiKey = options.apiKey;
    this.fetchImpl = options.fetch ?? globalThis.fetch.bind(globalThis);
    this.retry = options.retry === undefined ? {} : options.retry;
    this.timeoutMs = options.timeoutMs ?? 30_000;
  }

  globalStats(): Promise<JsonObject> {
    return this.get("/api/v1/stats/global");
  }

  /** Operation-named alias for {@link globalStats}. */
  getGlobalStats(): Promise<JsonObject> {
    return this.globalStats();
  }

  getContract(contractId: string): Promise<JsonObject> {
    return this.get(`/api/v1/contracts/${encodeURIComponent(contractId)}`);
  }

  getContractStats(contractId: string, window?: string): Promise<JsonObject> {
    return this.get(`/api/v1/contracts/${encodeURIComponent(contractId)}/stats`, { window });
  }

  listContracts(params: ListParams = {}): Promise<Page<JsonObject>> {
    return this.page("/api/v1/contracts", params, "contracts");
  }

  listEvents(contractId: string, params: ListParams = {}): Promise<Page<JsonObject>> {
    return this.page(`/api/v1/contracts/${encodeURIComponent(contractId)}/events`, params, "events");
  }

  listInvocations(contractId: string, params: ListParams = {}): Promise<Page<JsonObject>> {
    return this.page(
      `/api/v1/contracts/${encodeURIComponent(contractId)}/invocations`,
      params,
      "invocations",
    );
  }

  /** Async iterator over every contract, following `next_cursor`. */
  async *contracts(params: ListParams = {}): AsyncGenerator<JsonObject> {
    yield* this.paginate((cursor) => this.listContracts({ ...params, ...(cursor ? { cursor } : {}) }));
  }

  /** Operation-named alias for {@link listEvents}. */
  listContractEvents(contractId: string, params: ListParams = {}): Promise<Page<JsonObject>> {
    return this.listEvents(contractId, params);
  }

  /** Operation-named alias for {@link listInvocations}. */
  listContractInvocations(contractId: string, params: ListParams = {}): Promise<Page<JsonObject>> {
    return this.listInvocations(contractId, params);
  }

  /** Async iterator over every event of a contract. */
  async *events(contractId: string, params: ListParams = {}): AsyncGenerator<JsonObject> {
    yield* this.paginate((cursor) =>
      this.listEvents(contractId, { ...params, ...(cursor ? { cursor } : {}) }),
    );
  }

  /** Async iterator over every invocation of a contract. */
  async *invocations(contractId: string, params: ListParams = {}): AsyncGenerator<JsonObject> {
    yield* this.paginate((cursor) =>
      this.listInvocations(contractId, { ...params, ...(cursor ? { cursor } : {}) }),
    );
  }

  private async *paginate(fetchPage: (cursor?: string) => Promise<Page<JsonObject>>): AsyncGenerator<JsonObject> {
    let cursor: string | undefined;
    while (true) {
      const page = await fetchPage(cursor);
      yield* page.items;
      if (!page.nextCursor) return;
      cursor = page.nextCursor;
    }
  }

  /**
   * Live events from `GET /api/v1/stream/events`.
   *
   * Reconnects with exponential backoff, sends `Last-Event-ID` and skips ids it
   * has already yielded (at-least-once on the wire, at-most-once here for the
   * last 256 ids). A 401/403 ends the iteration with a `SorolensError`.
   */
  async *streamEvents(options: StreamOptions = {}): AsyncGenerator<EventEnvelope> {
    const params: ListParams = {};
    if (options.contractId) params.contract_id = options.contractId;
    const url = new URL(this.baseUrl + "/api/v1/stream/events");
    for (const [key, value] of Object.entries(params)) url.searchParams.set(key, String(value));

    const seen = new Set<string>();
    const order: string[] = [];
    let lastEventId = options.lastEventId;
    let attempt = 0;

    while (!options.signal?.aborted) {
      const controller = new AbortController();
      const onAbort = () => controller.abort();
      options.signal?.addEventListener("abort", onAbort, { once: true });
      let timer = setTimeout(
        () => controller.abort(),
        options.heartbeatTimeoutMs ?? 45_000,
      );
      try {
        const headers = this.headers();
        headers.Accept = "text/event-stream";
        if (lastEventId) headers["Last-Event-ID"] = lastEventId;
        const response = await this.fetchImpl(url, { headers, signal: controller.signal });
        if (!response.ok) {
          throw await this.error(response);
        }
        attempt = 0;
        const reader = response.body?.getReader();
        if (!reader) throw new Error("stream response has no body");
        const decoder = new TextDecoder();
        let buffer = "";
        let id = "";
        let name = "";
        let data: string[] = [];

        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          clearTimeout(timer);
          timer = setTimeout(() => controller.abort(), options.heartbeatTimeoutMs ?? 45_000);
          buffer += decoder.decode(value, { stream: true });
          let index = buffer.indexOf("\n");
          while (index !== -1) {
            const line = buffer.slice(0, index).replace(/\r$/, "");
            buffer = buffer.slice(index + 1);
            if (line === "") {
              if (data.length > 0) {
                const payload = data.join("\n");
                const envelope: EventEnvelope = { data: parseJson(payload) };
                if (id) envelope.id = id;
                if (name) envelope.type = name;
                if (!id || !seen.has(id)) {
                  if (id) {
                    seen.add(id);
                    order.push(id);
                    if (order.length > 256) {
                      const oldest = order.shift();
                      if (oldest) seen.delete(oldest);
                    }
                    lastEventId = id;
                  }
                  yield envelope;
                }
              }
              id = "";
              name = "";
              data = [];
            } else if (!line.startsWith(":")) {
              const colon = line.indexOf(":");
              const field = colon === -1 ? line : line.slice(0, colon);
              const value = colon === -1 ? "" : line.slice(colon + 1).replace(/^ /, "");
              if (field === "id") id = value;
              else if (field === "event") name = value;
              else if (field === "data") data.push(value);
            }
            index = buffer.indexOf("\n");
          }
        }
      } catch (error) {
        if (options.signal?.aborted) return;
        if (error instanceof SorolensError && error.permanent) throw error;
        const max = options.maxBackoffMs ?? 30_000;
        const base = Math.min((options.initialBackoffMs ?? 500) * 2 ** attempt, max);
        attempt += 1;
        await sleep(base / 2 + Math.random() * (base / 2));
      } finally {
        clearTimeout(timer);
        options.signal?.removeEventListener("abort", onAbort);
      }
    }
  }

  private async page(path: string, params: ListParams, key: string): Promise<Page<JsonObject>> {
    const body = await this.get(path, params);
    const items = Array.isArray(body[key]) ? (body[key] as JsonObject[]) : [];
    const next = body.next_cursor;
    return typeof next === "string" && next.length > 0 ? { items, nextCursor: next } : { items };
  }

  async get(path: string, params: ListParams = {}): Promise<JsonObject> {
    const response = await this.request(path, params);
    const text = await response.text();
    if (!text) return {};
    try {
      return JSON.parse(text) as JsonObject;
    } catch {
      throw new SorolensError({ status: response.status, message: "invalid JSON response", body: text });
    }
  }

  private async request(path: string, params: ListParams): Promise<Response> {
    const url = new URL(this.baseUrl + path);
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== "") url.searchParams.set(key, String(value));
    }

    const retry = this.retry === false ? undefined : { ...DEFAULT_RETRY, ...this.retry };
    let attempt = 0;
    while (true) {
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), this.timeoutMs);
      let response: Response;
      try {
        response = await this.fetchImpl(url, {
          method: "GET",
          headers: this.headers(),
          signal: controller.signal,
        });
      } catch (error) {
        clearTimeout(timer);
        if (retry && attempt < retry.maxRetries) {
          await sleep(this.delay(attempt, retry));
          attempt += 1;
          continue;
        }
        throw error;
      }
      clearTimeout(timer);

      if (response.ok) return response;

      const retryable = retry && retry.retryStatuses.includes(response.status);
      if (!retryable || attempt >= retry.maxRetries) {
        throw await this.error(response);
      }
      const retryAfter = parseRetryAfter(response.headers.get("Retry-After"));
      const delay = retryAfter ?? this.delay(attempt, retry);
      // Release the body before retrying; Workers/undici require it.
      await response.body?.cancel().catch(() => undefined);
      attempt += 1;
      await sleep(delay);
    }
  }

  private delay(attempt: number, retry: { baseDelayMs: number; maxDelayMs: number }): number {
    const base = Math.min(retry.baseDelayMs * 2 ** attempt, retry.maxDelayMs);
    // Full jitter: anywhere in [base/2, base].
    return base / 2 + Math.random() * (base / 2);
  }

  private headers(): Record<string, string> {
    const headers: Record<string, string> = { Accept: "application/json" };
    if (this.apiKey) headers.Authorization = `Bearer ${this.apiKey}`;
    return headers;
  }

  private async error(response: Response): Promise<SorolensError> {
    const body = await response.text().catch(() => "");
    let message = `HTTP ${response.status}`;
    let code: string | undefined;
    try {
      const parsed = JSON.parse(body) as { error?: unknown };
      if (typeof parsed.error === "string") {
        message = parsed.error;
      } else if (parsed.error && typeof parsed.error === "object") {
        const error = parsed.error as { code?: unknown; message?: unknown };
        if (typeof error.message === "string") message = error.message;
        if (typeof error.code === "string") code = error.code;
      }
    } catch {
      // Keep the status-only message.
    }
    return new SorolensError({ status: response.status, ...(code ? { code } : {}), message, body });
  }
}
