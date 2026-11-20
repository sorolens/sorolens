"use client";

import { useCallback, useEffect, useState } from "react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import {
  createMyApiKey,
  listMyApiKeys,
  revokeMyApiKey,
  type APIKey,
} from "@/lib/apiKeys";

/**
 * API-key card backed by `/api/v1/me/api-keys` (issue #613).
 *
 * Keys are created on the server and bound to this workspace's session user.
 * The plaintext token is returned once; it is kept in component memory so it
 * can be copied, and never written to localStorage. After a reload the card
 * shows only the server-issued prefix.
 */
export function ApiKeySection() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [plaintext, setPlaintext] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [revoking, setRevoking] = useState<string | null>(null);
  const [pendingRevokeId, setPendingRevokeId] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const load = useCallback(async () => {
    setError(null);
    try {
      setKeys(await listMyApiKeys());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load API keys.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function handleCreate() {
    setCreating(true);
    setError(null);
    setCopied(false);
    try {
      const created = await createMyApiKey();
      setKeys((current) => [created, ...current]);
      setPlaintext(created.key);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Could not create an API key."
      );
    } finally {
      setCreating(false);
    }
  }

  async function handleRevoke(id: string) {
    setRevoking(id);
    setError(null);
    try {
      await revokeMyApiKey(id);
      setKeys((current) => current.filter((key) => key.id !== id));
      setPlaintext(null);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Could not revoke the API key."
      );
    } finally {
      setRevoking(null);
    }
  }

  async function copy(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
    } catch {
      setError("Could not copy to the clipboard.");
    }
  }

  return (
    <section
      data-testid="settings-api-key"
      aria-labelledby="settings-api-key-heading"
      className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5"
    >
      <h2
        id="settings-api-key-heading"
        className="text-base font-semibold text-[var(--color-text-primary)]"
      >
        API keys
      </h2>
      <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
        Keys for calling the Sorolens API from your own scripts. They are bound
        to this workspace and can be revoked at any time.
      </p>

      <div className="mt-4 space-y-3">
        {loading ? (
          <p
            role="status"
            className="text-sm text-[var(--color-text-secondary)]"
          >
            Loading keys…
          </p>
        ) : keys.length === 0 ? (
          <p className="text-sm text-[var(--color-text-secondary)]">
            No API key yet. Create one to authenticate requests.
          </p>
        ) : (
          <ul className="space-y-2">
            {keys.map((key) => (
              <li
                key={key.id}
                data-testid="settings-api-key-row"
                className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-[var(--color-border)] px-3 py-2"
              >
                <div>
                  <p className="text-sm font-medium text-[var(--color-text-primary)]">
                    {key.name}
                  </p>
                  <p className="text-xs text-[var(--color-text-secondary)]">
                    Created {new Date(key.created_at).toLocaleDateString()}
                  </p>
                </div>
                <code className="rounded bg-[var(--color-bg-page)] px-2 py-1 font-mono text-xs text-[var(--color-text-primary)]">
                  {key.key_prefix}…
                </code>
                <button
                  type="button"
                  data-testid="settings-api-key-revoke"
                  disabled={revoking === key.id}
                  onClick={() => setPendingRevokeId(key.id)}
                  className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {revoking === key.id ? "Revoking…" : "Revoke"}
                </button>
              </li>
            ))}
          </ul>
        )}

        <ConfirmDialog
          isOpen={pendingRevokeId !== null}
          title="Revoke API key"
          description="Are you sure you want to revoke this API key? Any applications or scripts using it will lose access immediately."
          confirmText="Revoke key"
          isDestructive={true}
          isLoading={revoking !== null}
          onConfirm={async () => {
            if (pendingRevokeId) {
              await handleRevoke(pendingRevokeId);
              setPendingRevokeId(null);
            }
          }}
          onClose={() => setPendingRevokeId(null)}
        />

        {plaintext && (
          <div
            data-testid="settings-api-key-created"
            className="rounded-md border border-[var(--color-border)] bg-[var(--color-bg-page)] p-3"
          >
            <p className="text-xs text-[var(--color-text-secondary)]">
              Copy this key now — it will not be shown again.
            </p>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <code
                data-testid="settings-api-key-value"
                className="overflow-x-auto rounded bg-black/30 px-2 py-1 font-mono text-xs text-[var(--color-text-primary)]"
              >
                {plaintext}
              </code>
              <button
                type="button"
                data-testid="settings-api-key-copy"
                onClick={() => copy(plaintext)}
                className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)]"
              >
                {copied ? "Copied" : "Copy"}
              </button>
            </div>
          </div>
        )}

        {error && (
          <p
            data-testid="settings-api-key-error"
            role="alert"
            className="text-xs text-[var(--color-text-secondary)]"
          >
            {error}
          </p>
        )}
      </div>

      <div className="mt-5 flex items-center gap-3">
        <button
          type="button"
          onClick={handleCreate}
          disabled={creating || loading}
          className="rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {creating ? "Creating…" : "Generate key"}
        </button>
      </div>
    </section>
  );
}
