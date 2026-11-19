"use client";

import { useState } from "react";
import { createMyApiKey } from "@/lib/apiKeys";
import {
  EXTENSION_INSTALL_URL,
  EXTENSION_KEY_NAME,
  EXTENSION_KEY_SCOPES,
  EXTENSION_RELEASES_URL,
} from "@/lib/extension";

/**
 * Browser-extension card (issue #764).
 *
 * The extension used to ask users to open devtools and copy
 * `sorolens_user_id` out of localStorage. That identity no longer works: raw
 * `X-User-ID` headers stopped being trusted in issue #518, so the only
 * credentials the API accepts are a scoped API key or a signed session token.
 * A key is the better hand-off — it is scoped to the two endpoints the
 * extension calls, it is revocable from the API-keys card below, and it never
 * leaves the extension's `storage.local`.
 */
export function ExtensionSection() {
  const [key, setKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleCreate() {
    setBusy(true);
    setError(null);
    setCopied(false);
    try {
      const created = await createMyApiKey(EXTENSION_KEY_NAME, [
        ...EXTENSION_KEY_SCOPES,
      ]);
      setKey(created.key);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Could not create an extension key."
      );
    } finally {
      setBusy(false);
    }
  }

  async function handleCopy() {
    if (!key) return;
    try {
      await navigator.clipboard.writeText(key);
      setCopied(true);
    } catch {
      setError("Could not copy to the clipboard.");
    }
  }

  return (
    <section
      data-testid="settings-extension"
      aria-labelledby="settings-extension-heading"
      className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5"
    >
      <h2
        id="settings-extension-heading"
        className="text-base font-semibold text-[var(--color-text-primary)]"
      >
        Browser extension
      </h2>
      <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
        Puts a <strong>Track on Sorolens</strong> button beside every Soroban
        contract ID on Stellar Expert and Stellar Lab, tracks contracts from the
        toolbar, and notifies you when a watched contract stops responding.
      </p>

      <div className="mt-4 flex flex-wrap gap-3">
        <a
          href={EXTENSION_INSTALL_URL}
          className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)]"
          rel="noopener noreferrer"
        >
          Install instructions
        </a>
        <a
          href={EXTENSION_RELEASES_URL}
          className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)]"
          rel="noopener noreferrer"
        >
          Release builds
        </a>
      </div>

      <h3
        id="settings-extension-pairing-heading"
        className="mt-5 text-sm font-semibold text-[var(--color-text-primary)]"
      >
        Connect it to this account
      </h3>
      <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
        Create a key, paste it into the extension&apos;s Options page under{" "}
        <strong>API key</strong>, and the extension shares this workspace&apos;s
        watchlist. The key is scoped to {EXTENSION_KEY_SCOPES.join(" and ")} and
        shown once; revoke it any time from the API-keys card.
      </p>

      <div className="mt-3 flex flex-wrap items-center gap-2">
        <button
          type="button"
          data-testid="settings-extension-create"
          onClick={handleCreate}
          disabled={busy}
          className="rounded-md bg-[var(--color-accent)] px-3 py-1.5 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {busy
            ? "Creating…"
            : key
              ? "Create another key"
              : "Create extension key"}
        </button>
        {key && (
          <>
            <code
              data-testid="settings-extension-key"
              className="rounded bg-[var(--color-bg-page)] px-2 py-1 font-mono text-xs text-[var(--color-text-primary)]"
            >
              {key}
            </code>
            <button
              type="button"
              data-testid="settings-extension-copy"
              onClick={handleCopy}
              className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)]"
            >
              {copied ? "Copied" : "Copy"}
            </button>
          </>
        )}
      </div>

      {error && (
        <p
          role="alert"
          data-testid="settings-extension-error"
          className="mt-2 text-xs text-red-400"
        >
          {error}
        </p>
      )}
    </section>
  );
}
