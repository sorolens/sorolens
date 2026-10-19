"use client";

import { useState } from "react";
import { SettingsCard, type SaveState } from "./SettingsCard";
import { createApiKey, revokeApiKey, type CreatedApiKey } from "@/lib/apiKeys";

/**
 * API-key card, backed by the real API (issue #613).
 *
 * Generating a key is a POST /api/v1/api-keys; the server returns the
 * plaintext exactly once, it is kept in this component's memory for the
 * one-time copy, and only the key id and non-secret prefix are persisted.
 * Revoking calls DELETE on the server before clearing local state. The API
 * requires admin scope that a minted browser session does not have, so the
 * server's own 401/403 message is surfaced verbatim — the card never writes a
 * fabricated token into storage again.
 */
export function ApiKeySection({
  apiKeyId,
  apiKeyPrefix,
  apiKeyCreatedAt,
  userId,
  dirty,
  status,
  onChange,
  onSave,
}: {
  apiKeyId: string | null;
  apiKeyPrefix: string | null;
  apiKeyCreatedAt: string | null;
  /** Session user id; its bearer token authenticates the key calls. */
  userId: string;
  dirty: boolean;
  status: SaveState;
  onChange: (
    apiKeyId: string | null,
    apiKeyPrefix: string | null,
    apiKeyCreatedAt: string | null
  ) => void;
  onSave: () => boolean;
}) {
  // Plaintext from a successful creation. Component state only — never
  // written to settings, never re-read after a reload (issue #613).
  const [issued, setIssued] = useState<CreatedApiKey | null>(null);
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function generate() {
    setBusy(true);
    setError(null);
    try {
      const created = await createApiKey({
        name: "Sorolens settings key",
        scopes: ["read:contracts"],
        userId,
      });
      setIssued(created);
      setCopied(false);
      onChange(created.id, created.key_prefix, created.created_at);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "The API rejected the key request. Please try again."
      );
    } finally {
      setBusy(false);
    }
  }

  async function revoke() {
    setBusy(true);
    setError(null);
    try {
      if (apiKeyId) await revokeApiKey(apiKeyId, userId);
      onChange(null, null, null);
      setIssued(null);
      setCopied(false);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "The API rejected the revoke request. Please try again."
      );
    } finally {
      setBusy(false);
    }
  }

  async function copyKey() {
    if (!issued) return;
    await navigator.clipboard?.writeText(issued.key);
    setCopied(true);
  }

  return (
    <SettingsCard
      testId="settings-api-key"
      title="API key"
      description="A personal key for calling the Sorolens API from your own scripts. Keys are issued by the API and require admin scope; the full token is shown once at creation."
      status={status}
      onSave={onSave}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-sm font-medium text-[var(--color-text-primary)]">
            {apiKeyId ? "Active key" : "No API key"}
          </p>
          <p className="text-xs text-[var(--color-text-secondary)]">
            {apiKeyId && apiKeyCreatedAt
              ? `Created ${new Date(apiKeyCreatedAt).toLocaleDateString()}`
              : "Generate a key to authenticate requests."}
          </p>
        </div>

        <code
          data-testid="settings-api-key-value"
          className="rounded bg-[var(--color-bg-page)] px-2 py-1 font-mono text-xs text-[var(--color-text-primary)]"
        >
          {issued ? issued.key : apiKeyPrefix ? `${apiKeyPrefix}…` : "—"}
        </code>
      </div>

      <div className="flex flex-wrap items-center gap-2 border-t border-[var(--color-border)] pt-3">
        <button
          type="button"
          disabled={busy}
          onClick={() => {
            void generate();
          }}
          className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
        >
          {apiKeyId ? "Regenerate key" : "Generate key"}
        </button>

        <button
          type="button"
          disabled={!issued}
          onClick={() => {
            void copyKey();
          }}
          className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
        >
          {copied ? "Copied" : "Copy key"}
        </button>

        <button
          type="button"
          disabled={busy || !apiKeyId}
          onClick={() => {
            void revoke();
          }}
          className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
        >
          Revoke
        </button>

        {error && (
          <p
            data-testid="settings-api-key-error"
            role="alert"
            className="text-xs text-[var(--color-text-secondary)]"
          >
            {error}
          </p>
        )}

        {dirty && (
          <span
            data-testid="settings-api-key-unsaved"
            className="text-xs text-[var(--color-text-secondary)]"
          >
            Unsaved change — press Save to apply.
          </span>
        )}
      </div>
    </SettingsCard>
  );
}
