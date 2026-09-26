"use client";

import { useMemo, useState } from "react";
import { EndpointEntry, endpointsByTag } from "@/lib/openapi";
import {
  buildCurl,
  buildUrl,
  highlightJson,
  type ResponseState,
} from "@/lib/playground";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export default function PlaygroundPage() {
  const groups = useMemo(() => endpointsByTag(), []);
  const [selected, setSelected] = useState<EndpointEntry>(groups[0].endpoints[0]);
  const [pathParams, setPathParams] = useState<Record<string, string>>({});
  const [queryParams, setQueryParams] = useState<Record<string, string>>({});
  const [body, setBody] = useState(groups[0].endpoints[0].operation.requestBody?.example ?? "");
  const [apiKey, setApiKey] = useState("");
  const [response, setResponse] = useState<ResponseState | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);

  const curl = buildCurl(
    selected.method,
    buildUrl(API_URL, selected.path, pathParams, queryParams, selected.operation.parameters),
    apiKey,
    body,
  );

  function selectEndpoint(entry: EndpointEntry) {
    setSelected(entry);
    setPathParams({});
    setQueryParams({});
    setBody(entry.operation.requestBody?.example ?? "");
    setResponse(null);
    setError(null);
  }

  async function send() {
    setSending(true);
    setError(null);
    const url = buildUrl(
      API_URL,
      selected.path,
      pathParams,
      queryParams,
      selected.operation.parameters,
    );
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;

    const started = Date.now();
    try {
      const res = await fetch(url, {
        method: selected.method,
        headers,
        body: selected.method === "GET" ? undefined : body,
      });
      const text = await res.text();
      const headerPairs: [string, string][] = [];
      res.headers?.forEach?.((value: string, key: string) => {
        headerPairs.push([key, value]);
      });
      setResponse({
        status: res.status,
        statusText: res.statusText,
        headers: headerPairs,
        body: text,
        durationMs: Date.now() - started,
      });
    } catch (e) {
      setResponse(null);
      setError(e instanceof Error ? e.message : "Request failed");
    } finally {
      setSending(false);
    }
  }

  async function copyCurl() {
    try {
      await navigator.clipboard?.writeText(curl);
    } catch {
      // Clipboard may be unavailable (insecure context); the command is shown
      // in the DOM regardless.
    }
  }

  return (
    <div>
      <h1 className="mb-1 text-2xl font-semibold text-[var(--color-text-primary)]">
        API Playground
      </h1>
      <p className="mb-6 text-sm text-[var(--color-text-secondary)]">
        Pick an endpoint, fill in the parameters, and send a request. Your API
        key is never pre-filled.
      </p>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[240px_1fr_1fr]">
        {/* Endpoint tree */}
        <aside className="rounded-lg bg-[var(--color-bg-card)] p-3">
          {groups.map((group) => (
            <div key={group.tag} className="mb-3">
              <div className="mb-1 px-1 text-xs font-semibold uppercase tracking-wide text-[var(--color-text-secondary)]">
                {group.tag}
              </div>
              <ul>
                {group.endpoints.map((entry) => {
                  const active =
                    entry.method === selected.method && entry.path === selected.path;
                  return (
                    <li key={`${entry.method}-${entry.path}`}>
                      <button
                        type="button"
                        data-testid={`endpoint-${entry.method}-${entry.path}`}
                        onClick={() => selectEndpoint(entry)}
                        className={`w-full rounded-md px-2 py-1.5 text-left text-xs transition-colors ${
                          active
                            ? "bg-[var(--color-accent)] text-[var(--color-bg-page)]"
                            : "text-[var(--color-text-secondary)] hover:bg-white/5 hover:text-[var(--color-text-primary)]"
                        }`}
                      >
                        <span className="mr-2 font-mono font-semibold">
                          {entry.method}
                        </span>
                        <span className="font-mono">{entry.path}</span>
                        <span className="mt-0.5 block">{entry.operation.summary}</span>
                      </button>
                    </li>
                  );
                })}
              </ul>
            </div>
          ))}
        </aside>

        {/* Request form */}
        <section className="rounded-lg bg-[var(--color-bg-card)] p-4">
          <h2 className="mb-3 text-sm font-semibold text-[var(--color-text-primary)]">
            Request
          </h2>
          <div className="mb-3 font-mono text-xs text-[var(--color-text-secondary)]">
            {selected.method} {selected.path}
          </div>

          {selected.operation.parameters.length === 0 && (
            <p className="mb-3 text-xs text-[var(--color-text-secondary)]">
              This endpoint has no parameters.
            </p>
          )}

          {selected.operation.parameters.map((p) => (
            <div key={`${p.in}-${p.name}`} className="mb-3">
              <label
                htmlFor={`param-${p.name}`}
                className="mb-1 block text-xs font-medium text-[var(--color-text-secondary)]"
              >
                {p.name}
                <span className="ml-1 font-normal">({p.in})</span>
                {p.required && <span className="ml-1 text-red-400">*</span>}
              </label>
              <input
                id={`param-${p.name}`}
                value={
                  p.in === "path"
                    ? pathParams[p.name] ?? ""
                    : queryParams[p.name] ?? p.default ?? ""
                }
                onChange={(e) => {
                  const value = e.target.value;
                  if (p.in === "path") {
                    setPathParams((prev) => ({ ...prev, [p.name]: value }));
                  } else {
                    setQueryParams((prev) => ({ ...prev, [p.name]: value }));
                  }
                }}
                placeholder={p.description}
                className="w-full rounded-md border border-[var(--color-border)] bg-black/30 px-3 py-2 font-mono text-xs text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
              />
            </div>
          ))}

          {selected.operation.requestBody && (
            <div className="mb-3">
              <label
                htmlFor="request-body"
                className="mb-1 block text-xs font-medium text-[var(--color-text-secondary)]"
              >
                Request body (JSON)
              </label>
              <textarea
                id="request-body"
                value={body}
                onChange={(e) => setBody(e.target.value)}
                rows={6}
                spellCheck={false}
                className="w-full rounded-md border border-[var(--color-border)] bg-black/30 px-3 py-2 font-mono text-xs text-[var(--color-text-primary)] focus:border-[var(--color-accent)] focus:outline-none"
              />
            </div>
          )}

          <div className="mb-4">
            <label
              htmlFor="playground-api-key"
              className="mb-1 block text-xs font-medium text-[var(--color-text-secondary)]"
            >
              API key (optional, not stored)
            </label>
            <input
              id="playground-api-key"
              type="password"
              autoComplete="off"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder="Paste your key…"
              className="w-full rounded-md border border-[var(--color-border)] bg-black/30 px-3 py-2 font-mono text-xs text-[var(--color-text-primary)] placeholder-[var(--color-text-secondary)] focus:border-[var(--color-accent)] focus:outline-none"
            />
          </div>

          <div className="flex flex-wrap gap-2">
            <button
              id="playground-send"
              type="button"
              onClick={send}
              disabled={sending}
              className="rounded-lg bg-[var(--color-accent)] px-4 py-2 text-sm font-semibold text-[var(--color-bg-page)] transition-opacity hover:opacity-90 disabled:opacity-50"
            >
              {sending ? "Sending…" : "Send"}
            </button>
            <button
              id="playground-copy-curl"
              type="button"
              onClick={copyCurl}
              className="rounded-lg border border-[var(--color-border)] px-4 py-2 text-sm font-medium text-[var(--color-text-secondary)] transition-colors hover:text-[var(--color-text-primary)]"
            >
              Copy as curl
            </button>
          </div>

          <div className="mt-4">
            <div className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
              curl
            </div>
            <pre
              data-testid="curl-command"
              className="overflow-x-auto rounded-md bg-black/30 p-3 font-mono text-[11px] leading-relaxed"
            >
              {curl}
            </pre>
          </div>
        </section>

        {/* Response viewer */}
        <section className="rounded-lg bg-[var(--color-bg-card)] p-4">
          <h2 className="mb-3 text-sm font-semibold text-[var(--color-text-primary)]">
            Response
          </h2>

          {!response && !error && (
            <p className="text-xs text-[var(--color-text-secondary)]">
              Send a request to see the status, headers, and body.
            </p>
          )}

          {error && (
            <p role="alert" className="text-sm text-[var(--color-danger)]">
              {error}
            </p>
          )}

          {response && (
            <div className="space-y-3">
              <div className="flex items-center gap-3">
                <span
                  data-testid="response-status"
                  className={`rounded-full px-2.5 py-0.5 text-xs font-semibold ${
                    response.status >= 200 && response.status < 300
                      ? "bg-green-900/40 text-green-400"
                      : "bg-red-900/40 text-red-400"
                  }`}
                >
                  {response.status} {response.statusText}
                </span>
                <span className="text-xs text-[var(--color-text-secondary)]">
                  {response.durationMs} ms
                </span>
              </div>

              {response.headers.length > 0 && (
                <div>
                  <div className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
                    Headers
                  </div>
                  <pre className="overflow-x-auto rounded-md bg-black/20 p-3 font-mono text-[11px]">
                    {response.headers
                      .map(([k, v]) => `${k}: ${v}`)
                      .join("\n")}
                  </pre>
                </div>
              )}

              <div>
                <div className="mb-1 text-xs font-medium text-[var(--color-text-secondary)]">
                  Body
                </div>
                <pre
                  data-testid="response-body"
                  className="max-h-96 overflow-auto rounded-md bg-black/20 p-3 font-mono text-[11px] leading-relaxed"
                  dangerouslySetInnerHTML={{ __html: highlightJson(response.body) }}
                />
              </div>
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
