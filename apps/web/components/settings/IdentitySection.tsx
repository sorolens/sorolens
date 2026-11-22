"use client";

import { useEffect, useState } from "react";
import { ensureSession } from "@/lib/user";

/**
 * Shows the workspace user ID minted by `POST /api/v1/auth/session` and a
 * copy button, so a user can identify the account an API key is bound to
 * (issue #613). The ID is read from the verified session, never asserted.
 */
export function IdentitySection() {
  const [userId, setUserId] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    let active = true;
    ensureSession()
      .then((session) => {
        if (active) setUserId(session.user_id);
      })
      .catch((err: unknown) => {
        if (active) {
          setError(
            err instanceof Error ? err.message : "Could not load your user ID."
          );
        }
      });
    return () => {
      active = false;
    };
  }, []);

  async function copy() {
    if (!userId) return;
    try {
      await navigator.clipboard.writeText(userId);
      setCopied(true);
    } catch {
      setError("Could not copy to the clipboard.");
    }
  }

  return (
    <section
      data-testid="settings-identity"
      aria-labelledby="settings-identity-heading"
      className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5"
    >
      <h2
        id="settings-identity-heading"
        className="text-base font-semibold text-[var(--color-text-primary)]"
      >
        Your user ID
      </h2>
      <p className="mt-1 text-sm text-[var(--color-text-secondary)]">
        The workspace identity API keys are bound to. Share it when asking an
        admin to grant your account a role.
      </p>

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <code
          data-testid="settings-user-id"
          className="rounded bg-[var(--color-bg-page)] px-2 py-1 font-mono text-xs text-[var(--color-text-primary)]"
        >
          {userId || (error ? "—" : "Loading…")}
        </code>
        <button
          type="button"
          data-testid="settings-user-id-copy"
          disabled={!userId}
          onClick={copy}
          className="rounded-md border border-[var(--color-border)] px-3 py-1.5 text-sm text-[var(--color-text-primary)] transition-colors hover:border-[var(--color-accent)] disabled:cursor-not-allowed disabled:opacity-50"
        >
          {copied ? "Copied" : "Copy"}
        </button>
      </div>

      {error && (
        <p
          role="alert"
          className="mt-2 text-xs text-[var(--color-text-secondary)]"
        >
          {error}
        </p>
      )}
    </section>
  );
}
