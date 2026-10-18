"use client";

import { useState } from "react";
import { SettingsCard, type SaveState } from "./SettingsCard";
import { generateApiKey, maskApiKey } from "@/lib/settings";

/**
 * API-key card. Generating or revoking only stages the change; Save commits it,
 * so a mis-click can be abandoned before it is persisted. The full token is
 * shown once on generation so it can be copied before it is masked away.
 */
export function ApiKeySection({
  apiKey,
  apiKeyCreatedAt,
  dirty,
  status,
  onChange,
  onSave,
}: {
  apiKey: string | null;
  apiKeyCreatedAt: string | null;
  dirty: boolean;
  status: SaveState;
  onChange: (apiKey: string | null, apiKeyCreatedAt: string | null) => void;
  onSave: () => boolean;
}) {
  const [revealed, setRevealed] = useState(false);

  return (
    <SettingsCard
      testId="settings-api-key"
      title="API key"
      description="A personal key for calling the Sorolens API from your own scripts."
      status={status}
      onSave={onSave}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-sm font-medium text-[var(--color-text-primary)]">
            {apiKey ? "Active key" : "No API key"}
          </p>
          <p className="text-xs text-[var(--color-text-secondary)]">
            {apiKey && apiKeyCreatedAt
              ? `Created ${new Date(apiKeyCreatedAt).toLocaleDateString()}`
              : "Generate a key to authenticate requests."}
          </p>
        </div>

        <code
          data-testid="settings-api-key-value"
          className="rounded bg-[var(--color-bg-page)] px-2 py-1 font-mono text-xs text-[var(--color-text-primary)]"
        >
          {apiKey ? (revealed ? apiKey : maskApiKey(apiKey)) : "—"}
        </code>
      </div>

      <div className="flex flex-wrap items-center gap-2 border-t border-[var(--color-border)] pt-3">
        <button
          type="button"
          onClick={() => {
            onChange(generateApiKey(), new Date().toISOString());
            setRevealed(true);
          }}
          className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)]"
        >
          {apiKey ? "Regenerate key" : "Generate key"}
        </button>

        <button
          type="button"
          disabled={!apiKey}
          onClick={() => setRevealed((current) => !current)}
          className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
        >
          {revealed ? "Hide" : "Reveal"}
        </button>

        <button
          type="button"
          disabled={!apiKey}
          onClick={() => {
            onChange(null, null);
            setRevealed(false);
          }}
          className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
        >
          Revoke
        </button>

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
